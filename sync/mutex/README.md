```
type Mutex struct {
    state int32
    sema  uint32
}
```

`sema`被用于进行各个goroutines之间的通信 （唤醒goroutine去获取锁资源）

`state`被用于表示状态，其中第一位为上锁状态，第二位为唤醒状态，第三位为饥饿状态, 其余位置代表正在等待获取锁资源的goroutines数量。  
* 上锁状态：即锁资源已经被获取了
* 唤醒状态：表明目前有处于马上可执行的goroutine正在排队等待中，无需通过sema来唤醒goroutines
* 饥饿状态：正在运行的goroutines会有更大的机会能够抢占到锁。等待时间超过`starvationThresholdNs`(1ms)的goroutines会进入的饥饿状态，可以优先抢占到锁，其他尝试获取锁的goroutines会排到末尾。（没有这个机制的话，可能前面沉睡的goroutine会长时间无法获取到锁）

```
const (
    mutexLocked = 1 << iota // mutex is locked
    mutexWoken
    mutexStarving
    mutexWaiterShift = iota

    starvationThresholdNs = 1e6
)
```

上锁操作
* 锁未被获取，直接获取即可
* 如果已经被获取了, 则使用`lockSlow`去排队等待获取资源
```
func (m *Mutex) Lock() {
    if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
        if race.Enabled {
            race.Acquire(unsafe.Pointer(m))
        }
        return
    }
    m.lockSlow()
}
```

其中`race.Enabled`部分为go自带的race-detector行为，先忽略（后面的关于race-detector的也全部省略了）

`lockSlow`
* 初始化一些变量
* 开始`for { if getLock: break; else: keep getting }`这样的获取锁过程

`自旋`：在一定条件下没必要让goroutine进入沉睡，等待sema，可以一直占有cpu等待其余goroutine释放锁。饥饿模式下没必要自旋反正也拿不到锁。
```
if old&(mutexLocked|mutexStarving) == mutexLocked && runtime_canSpin(iter) {
    if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
        atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
        awoke = true
    }
    runtime_doSpin()
    iter++
    old = m.state
    continue
}
```
* 进入自旋时顺便把woken设为1，避免唤醒其他goroutine。
* `runtime_canSpin(iter)` 具体细节？啥时候没必要沉睡?

`非自旋时`
* step1: 检查是否starving，不获取starving的锁
* step2：如果无法获取锁，则将等待锁的goroutine数量+1
* step3：在`unlock`时，我们认为starving的锁存在waiter，如果该goroutine设为的starving且没有waiter，那撤回starving的操作
* step4：设置woken状态
* step5：如果已经带待过了，那就插入队头，反之插入队尾。等待sema，并检查是否需要设为starving
* step6: 如果是该goroutine设置的starving则获取锁。判断是否要退出starving, 因为starving非常不效率。
```
new := old

// step 1
if old&mutexStarving == 0 {
    new |= mutexLocked
}

// step2
if old&(mutexLocked|mutexStarving) != 0 {
    new += 1 << mutexWaiterShift
}

// step3
if starving && old&mutexLocked != 0 {
    new |= mutexStarving
}

// step4 
if awoke {
    if new&mutexWoken == 0 {
        throw("sync: inconsistent mutex state")
    }
    new &^= mutexWoken
}


if atomic.CompareAndSwapInt32(&m.state, old, new) {
    if old&(mutexLocked|mutexStarving) == 0 {
        break
    }
    // step5
    queueLifo := waitStartTime != 0
    if waitStartTime == 0 {
        waitStartTime = runtime_nanotime()
    }
    runtime_SemacquireMutex(&m.sema, queueLifo, 1)
    starving = starving || runtime_nanotime()-waitStartTime > starvationThresholdNs
    old = m.state
    
    // step6
    if old&mutexStarving != 0 {
        if old&(mutexLocked|mutexWoken) != 0 || old>>mutexWaiterShift == 0 {
            throw("sync: inconsistent mutex state")
        }
        delta := int32(mutexLocked - 1<<mutexWaiterShift)
        if !starving || old>>mutexWaiterShift == 1 {
            delta -= mutexStarving
        }
        atomic.AddInt32(&m.state, delta)
        break
    }
    awoke = true
    iter = 0
} else {
    old = m.state
}
```
* TODO 为什么step6中` if !starving || old>>mutexWaiterShift == 1` 中需要判断`!starving`，能进入step6不是一定`starving`了吗？    
    g1 使mutex进入 starving, g1结束后并不会恢复。只有在`(1) it is the last waiter in the queue, or (2) it waited for less than 1 ms`下才会恢复到normal。
* TODO: search some info about `sync: inconsistent mutex state`

 解锁操作
* step1: 解锁
* step2: 判断是否需要发送sema去唤醒goroutine (锁已经被占了，进入了starving模式，进入了唤醒模式)
* step3: 唤醒goroutine
* step4: (饥饿模式下)直接给waiter, waiter会设置Lock `delta := int32(mutexLocked - 1<<mutexWaiterShift)`, mutexLocked就是用来设置Locked位的。


 `Unlock`
 ```
// step1 
new := atomic.AddInt32(&m.state, -mutexLocked)
if new != 0 {
    m.unlockSlow(new)
}
 ```


 `unlockSlow`
 ```
if (new+mutexLocked)&mutexLocked == 0 {
    throw("sync: unlock of unlocked mutex")
}
if new&mutexStarving == 0 {
    old := new
    for {
        // step2
        if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken|mutexStarving) != 0 {
            return
        }
        // step3
        new = (old - 1<<mutexWaiterShift) | mutexWoken
        if atomic.CompareAndSwapInt32(&m.state, old, new) {
            runtime_Semrelease(&m.sema, false, 1)
            return
        }
        old = m.state
    }
} else {
    // step4
    runtime_Semrelease(&m.sema, true, 1)
}
 ```

 * 不要复制mutex, copy了state和sema值，但是没有同样的解锁等操作作用在上面了。
 * CAS
```
old := m.state
new := old
// do sth with new
CAS(&m.state, old, new)
// failed: roll back; succeed: keep go on;
```
* starving主要是避免一个goroutine霸占一个锁，其他等待1ms后强行似的goroutine之间可以转换。