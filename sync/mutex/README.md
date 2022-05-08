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

`自旋`：在一定条件下没必要让goroutine进入沉睡，等待sema，可以一直占有cpu等待其余goroutine释放锁。
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

