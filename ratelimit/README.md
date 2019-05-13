在开发高并发系统时，为了保证系统的高可用和稳定性，业内流传着以下三种方案
- 缓存：提升系统访问速度，增大系统处理容量
- 降级：当服务出现故障或影响到核心业务时，暂时屏蔽掉，待高峰过后或故障解决后再打开
- 限流：通过对并发（或者一定时间窗口内）请求进行限速来保护系统，一旦达到限制速率则拒绝服务(定向到错误页或告知资源没有了)，排队等待（比如秒杀，评论，下单），降级（返回兜底数据或默认数据）

以下着重介绍下**限流**相关的技术、一般来说，限流的常用处理手段有：
- 计数器
- 滑动窗口
- 漏桶算法
- 令牌桶算法

### 计数器
计数器是一种比较简单粗暴的限流算法：
>在一段时间间隔内，对请求进行计数，与阈值进行比较判断是否需要限流，一旦到了时间临界点，将计数器清零
以下讨论两种通用做法:
#### 方案一
当请求频率过快时，直接抛弃后续请求
```golang
/**
限流方式-计数器
在一段时间间隔内，对请求进行计数，与阈值进行判断是否需要限流，一旦到了时间临界点，将计数器清零

planA: 当请求频率过快时,直接抛弃后续请求
 */
package main

import (
	"time"
	"sync"
	"fmt"
)

type LimitRate struct {
	rate int  			//计数周期内最多允许的请求数 (可以等于)
	begin time.Time		//计数开始时间
	cycle time.Duration	//计数周期
	count int 			//计数周期内累计收到的请求数
	lock sync.Mutex
}

func (limit *LimitRate) Create(r int, cycle time.Duration) {
	limit.rate = r
	limit.begin = time.Now()
	limit.cycle = cycle
	limit.count = 0
}

func (limit *LimitRate) Reset() {
	limit.begin = time.Now()
	limit.count = 0
}

func (limit *LimitRate) Allow() bool {
	//(1)并发下需要加锁,保证协程安全操作
	limit.lock.Lock()
	defer limit.lock.Unlock()

	//(2)计数的个数大于或等于计数周期内最多允许的请求数时,需要判断是否到达了计数周期
	if limit.count >= limit.rate {
		now := time.Now()
		//(2.1)当前时间距离上次重置时的时间间隔大于或等于计数周期时需要重置计数器
		if now.Sub(limit.begin) >= limit.cycle {
			limit.Reset()
			return false
		//(2.2)小于计数周期的话,直接过滤掉该请求
		} else {
			return false
		}
	} else {
		limit.count++
		return true
	}
}

func main() {
	var limitEg LimitRate
	limitEg.Create(2, time.Second)	// 1s内只允许两个请求通过
	wg := sync.WaitGroup{}
	for i:=0; i<10; i++ {
		wg.Add(1)

		//fmt.Println("Create req", i, time.Now())
		go func(i int) {
			if limitEg.Allow() {
				fmt.Println("Reponse req", i, time.Now())
			}
			wg.Done()
		}(i)
		time.Sleep(200 * time.Millisecond)
	}
	wg.Wait()
}

```
例子里面每200ms创建一个请求，总共10次请求，耗时2s。限制的速度是1s内最多2个请求，所以最多可以收到4次Reponse，以下为运行结果
```shell
Respon req 0 2019-05-08 10:00:21.139516 +0800 CST m=+0.000274006
Respon req 1 2019-05-08 10:00:21.343573 +0800 CST m=+0.204411976
Respon req 6 2019-05-08 10:00:22.358722 +0800 CST m=+1.219560566
Respon req 7 2019-05-08 10:00:22.560806 +0800 CST m=+1.421644153
```

#### 方案二
当请求频率太快时,后续的请求等待之前的请求完成后才进行，这里需要稍微修改一下**Allow**方法
```golang
func (limit *LimitRate) Allow() bool {
	//(1)并发下需要加锁,保证协程安全操作
	limit.lock.Lock()
	defer limit.lock.Unlock()

	//(2)计数的个数大于或等于计数周期内最多允许的请求数时,需要判断是否到达了计数周期
	if limit.count >= limit.rate {
		//循环等待请求,避免将请求直接丢弃
		for {
			now := time.Now()
			//(2.1)当前时间距离上次重置时的时间间隔大于或等于计数周期时需要重置计数器
			if now.Sub(limit.begin) >= limit.cycle {
				limit.Reset()
				break
			}
		}
	}
	//(3)无论如何都需要计数器自增
	limit.count++
	return true
}
```
限制的速度是1s内做多2个请求，10个请求预计耗时5s左右，以下为验证的结果
```
Respon req 0 2019-05-08 10:13:57.318168 +0800 CST m=+0.000341149
Respon req 1 2019-05-08 10:13:57.521002 +0800 CST m=+0.203175815
Respon req 2 2019-05-08 10:13:58.318126 +0800 CST m=+1.000299775
Respon req 3 2019-05-08 10:13:58.318172 +0800 CST m=+1.000345387
Respon req 4 2019-05-08 10:13:59.318045 +0800 CST m=+2.000299115
Respon req 5 2019-05-08 10:13:59.318086 +0800 CST m=+2.000340754
Respon req 6 2019-05-08 10:14:00.317965 +0800 CST m=+3.000299353
Respon req 7 2019-05-08 10:14:00.318037 +0800 CST m=+3.000371595
Respon req 8 2019-05-08 10:14:01.317965 +0800 CST m=+4.000299115
Respon req 9 2019-05-08 10:14:01.318008 +0800 CST m=+4.000342604

real    0m4.460s
user    0m3.803s
sys     0m0.271s

```
计数器算法存在**时间临界点**缺陷。比如每一分钟限速100个请求（也就是每秒最多1.7个请求），在00:00:00到00:00:58这段时间内没有任何用户请求，然后在00:00:59这一瞬时发出了100个请求，这是允许的，然后在00:01:00这一瞬时又发出了100个请求，短短1s内发出了200个请求，系统可能会承受恶意用户的大量请求，甚至击穿系统

### 滑动窗口
针对计数器存在的临界点缺陷
>滑动窗口把固定时间片进行划分，并且随着时间的流逝，进行移动，固定数量的可以移动的格子，进行计数并判断阈值

格子的数量影响着滑动窗口算法的精度，依然后时间片的概念，无法根本解决临界点的问题

### 漏桶算法
漏桶算法描述如下:
- 一个固定容量的漏桶，按照固定速率流出水滴
- 如果桶是空的，则不需要流出水滴
- 可以以任意速率流入水滴到漏桶
- 如果流入水滴超过了桶的容量，则溢出（被丢弃）

![leaky-bucket.png](./image/leaky-bucket.png)

漏桶算法思路很简单，水（请求）先进入漏桶里，漏桶以一定的速度出水，当水流速度过大会直接溢出，可以看出漏桶算法能强行限制数据的传输速率。通俗点来说，我们有一个固定容量的桶，有水流进来，也有水流出去。对于流进来的水（请求）来说，我们无法预计一共有多少水会流进来，也无法预计水流的速度。但是对于流出去的水来说，这个桶可以固定水流出的速率（处理速度），从而达到**流量整形(Traffic Shaping)**和**流量控制(Traffic Policing)**的效果
```golang
/**
漏桶算法思路很简单，水（请求）先进入到漏桶里，漏桶以一定的速度出水，当水流入速度过大会直接溢出，可以看出漏桶算法能强行限制数据的传输速率
*/
package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type LeakyBucket struct {
	rate        float64 //固定每秒出水速率
	capacity    float64 //桶的容量
	water       float64 //桶中当前水量
	lastLeakyMs int64   //桶上次漏水时间戳 ms

	lock sync.Mutex
}

func (leaky *LeakyBucket) Create(rate float64, capacity float64) {
	leaky.rate = rate
	leaky.capacity = capacity
	leaky.water = 0
	// int64 / float64
	leaky.lastLeakyMs = time.Now().UnixNano() / 1e6
}

func (Leaky *LeakyBucket) Allow() bool {
	Leaky.lock.Lock()
	defer Leaky.lock.Unlock()

	//(1)先执行漏水的操作
	now := time.Now().UnixNano() / 1e6 //ms
	expand := float64((now - Leaky.lastLeakyMs)) * Leaky.rate / 1000
	//(2)计算剩余水量,需要判断桶是否已经干了
	Leaky.water = Leaky.water - expand
	Leaky.water = math.Max(0, Leaky.water)

	Leaky.lastLeakyMs = now

	//fmt.Println(Leaky.water)
	//(3)如果新加的水量之后桶中当前水量大于桶的容量表示水满执行溢出操作,反之执行添加水操作
	if Leaky.water+1 > Leaky.capacity {
		return false
	} else {
		Leaky.water++
		return true
	}
}

func main() {
	var leaky LeakyBucket
	leaky.Create(3, 3)

	wg := sync.WaitGroup{}

	for i := 0; i < 30; i++ {
		wg.Add(1)

		//fmt.Println("Create req", i, time.Now())
		go func(i int) {
			if leaky.Allow() {
				fmt.Println("Reponse req", i, time.Now())
			}
			wg.Done()
		}(i)
		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
}

```
这里初始化桶容量为3，桶内无水，1s内创建了10个请求，随着请求往里面"滴水"，桶按照每秒3个单位的速率流出水，处理时，部分请求呗丢弃
```
Reponse req 0 2019-05-13 14:03:06.216637 +0800 CST m=+0.000237042
Reponse req 1 2019-05-13 14:03:06.319163 +0800 CST m=+0.102763143
Reponse req 2 2019-05-13 14:03:06.422699 +0800 CST m=+0.206299014
Reponse req 4 2019-05-13 14:03:06.631601 +0800 CST m=+0.415200416
Reponse req 7 2019-05-13 14:03:06.941607 +0800 CST m=+0.725206444
Reponse req 10 2019-05-13 14:03:07.255262 +0800 CST m=+1.038861658
Reponse req 13 2019-05-13 14:03:07.568576 +0800 CST m=+1.352255434
Reponse req 17 2019-05-13 14:03:07.986466 +0800 CST m=+1.770146368
Reponse req 20 2019-05-13 14:03:08.300034 +0800 CST m=+2.083714064
Reponse req 23 2019-05-13 14:03:08.61342 +0800 CST m=+2.397099486
Reponse req 26 2019-05-13 14:03:08.927519 +0800 CST m=+2.711280106
Reponse req 29 2019-05-13 14:03:09.240306 +0800 CST m=+3.024067248
```


### 令牌桶算法
由于漏桶出水速度时恒定的，如果瞬时爆发大流量的话，将有大部分请求被丢弃掉（溢出）。为了解决这个问题，产生了令牌桶算法。令牌桶算法描述如下：
- 有一个固定容量的桶，桶一开始是空的
- 以固定的速率**r**往tong桶里填充**token**，直到达到桶的容量，多余的令牌将会丢弃
- 每当一个请求过来时，就尝试从桶里移除一个令牌，如果没有令牌的话，请求无法通过

![token-bucket.jpg](./image/token-bucket.jpg)



### 漏桶算法和令牌桶算法对比
- 令牌桶是按照固定速率往桶中添加令牌，请求是否被处理需要看桶中令牌是否足够，当令牌数减为0时则拒绝新的请求
- 漏桶则是按照常量固定速率流出请求，流入请求速率任意，当流入的请求数累计到漏桶容量时，则新流入的请求被拒绝
- 令牌桶限制的是平均流入速率（允许突发请求，只要有令牌就可以处理，支持一次拿3个令牌，4个令牌），并允许一定程度的突发流量
- 漏桶限制的是常量流出速率（即流出速率是一个固定常量值，比如都是1的速率流出，而不能一次是1，下次是2），从而平滑突发流入速率
- 令牌桶允许一定程度的突发，而漏桶主要目的是平滑流入速率
- 两个算法实现可以一样，但是方向是相反的，对于相同的参数得到的限流效果是一样的