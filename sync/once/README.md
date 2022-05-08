去掉所有注释后，源码很短
```
type Once struct {
	done uint32 // Why first in struct?
	m    Mutex
}

func (o *Once) Do(f func()) {
    if atomic.LoadUint32(&o.done) == 0 {
        o.doSlow(f)
    }
}

func (o *Once) doSlow(f func()) {
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
	    defer atomic.StoreUint32(&o.done, 1) // Why defer?
		f()
	}
}
``` 
   
### 大体思路
* 用done表示是否f已经完成，若完成直接返回，反之则执行doSlow
* doSlow先获取锁资源，保证只有一个goroutine可以执行f
* 执行前再检查done （可能会有多个goroutine进入到doSlow中，但是只有一个真正能执行f)
* 执行后set done to 1 （保证once.Do返回前f已经被某一个goroutine执行完了）

注释回答了如下几个问题以及一些注意事项：

### 为什么done是Once的第一个member
`hot path is inlined at every call site` 和struct的ptr一样, 不用做计算去获取它的地址。

### 为什么需要在执行完f后再set done为1
保证在f完成前，其他goroutine会被doSlow中m.Lock()堵塞住。若提前赋值了done为1，那就存在某些goroutine可能在Do中直接返回，但是f尚未完成。

### f的注意事项
* f没有入参也没有出参，需要从外部获取一些var来进行初始化啥的。
* f内不要再调用once.Do，不然会死锁。