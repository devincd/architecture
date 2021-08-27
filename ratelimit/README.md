在开发高并发系统时，为了保证系统的高可用和稳定性，业内流传着以下三种方案
- 缓存：提升系统访问速度，增大系统处理容量
- 降级：当服务出现故障或影响到核心业务时，暂时屏蔽掉，待高峰过后或故障解决后再打开
- 限流：通过对并发（或者一定时间窗口内）请求进行限速来保护系统，一旦达到限制速率则拒绝服务(定向到错误页或告知资源没有了)，排队等待（比如秒杀，评论，下单），降级（返回兜底数据或默认数据）

以下着重介绍下**限流**相关的技术、一般来说，限流的常用处理手段有：
- 计数器
- 滑动窗口
- 漏桶算法
- 令牌桶算法

### 1.计数器
计数器是一种比较简单粗暴的限流算法：
>在一段时间间隔内，对请求进行计数，与阈值进行比较判断是否需要限流，一旦到了时间临界点，将计数器清零
以下讨论两种通用做法:
#### 方案一
当请求频率过快时，直接抛弃后续请求
```golang
/**
限流方式-计数器
在一段时间间隔内，对请求进行计数，与阈值进行判断是否需要限流，一旦到了时间临界点，将计数器清零。

planA: 当请求频率过快时,直接抛弃后续请求
*/
package main

import (
	"fmt"
	"sync"
	"time"
)

type LimitRate struct {
	rate  int           //计数周期内最多允许的请求数 (可以等于)
	begin time.Time     //计数开始时间
	cycle time.Duration //计数周期
	count int           //计数周期内累计收到的请求数
	lock  sync.Mutex
}

func (limit *LimitRate) Create(r int, cycle time.Duration) {
	limit.rate = r
	limit.begin = time.Now()
	limit.cycle = cycle //初始化计数周期,也就是在该时间段内最多允许rate中请求透过
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

	//(2)计数的个数(计数提前已经加过1)大于计数周期内最多允许的请求数时,需要判断是否到达了计数周期
	if limit.count+1 > limit.rate {
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
	var limiter LimitRate
	limiter.Create(2, time.Second) // 1s内只允许两个请求通过
	wg := sync.WaitGroup{}
	// 模拟在 2s 内发送20个请求，正常情况下有4个请求通过
	for i := 1; i <= 20; i++ {
		wg.Add(1)
		go func(i int) {
			if limiter.Allow() {
				fmt.Println("Response req", i, time.Now())
			}
			wg.Done()
		}(i)
		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
}
```
例子里面每100ms创建一个请求，总共20次请求，耗时2s。限制的速度是1s内最多2个请求，所以最多可以收到4次Response，以下为运行结果:
```shell
Response req 1 2021-08-27 10:58:13.831596 +0800 CST m=+0.000336018
Response req 2 2021-08-27 10:58:13.936241 +0800 CST m=+0.104981751
Response req 12 2021-08-27 10:58:14.965243 +0800 CST m=+1.133992389
Response req 13 2021-08-27 10:58:15.069271 +0800 CST m=+1.238020186
```

#### 方案二
当请求频率太快时,后续的请求等待之前的请求完成后才进行，这里需要稍微修改一下 **Allow** 方法。
```golang
func (limit *LimitRate) Allow() bool {
	//(1)并发下需要加锁,保证协程安全操作
    limit.lock.Lock()
    defer limit.lock.Unlock()

    //(2)计数的个数(计数提前已经加过1)大于计数周期内最多允许的请求数时,需要判断是否到达了计数周期
    if limit.count+1 > limit.rate {
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
例子里面每100ms创建一个请求，总共20次请求，耗时2s。限制的速度是1s内最多2个请求，所以最多可以收到20次Response，但是耗时10s,以下为运行结果:
```shell
Response req 1 2021-08-27 11:01:28.604854 +0800 CST m=+0.000241569
Response req 2 2021-08-27 11:01:28.709166 +0800 CST m=+0.104554366
Response req 3 2021-08-27 11:01:29.60482 +0800 CST m=+1.000214712
Response req 4 2021-08-27 11:01:29.60486 +0800 CST m=+1.000254239
Response req 5 2021-08-27 11:01:30.604809 +0800 CST m=+2.000209879
Response req 6 2021-08-27 11:01:30.604842 +0800 CST m=+2.000242591
Response req 7 2021-08-27 11:01:31.604803 +0800 CST m=+3.000210503
Response req 8 2021-08-27 11:01:31.604832 +0800 CST m=+3.000239866
Response req 9 2021-08-27 11:01:32.604797 +0800 CST m=+4.000211440
Response req 10 2021-08-27 11:01:32.604822 +0800 CST m=+4.000236953
Response req 11 2021-08-27 11:01:33.604789 +0800 CST m=+5.000210250
Response req 12 2021-08-27 11:01:33.60482 +0800 CST m=+5.000241864
Response req 13 2021-08-27 11:01:34.604783 +0800 CST m=+6.000211564
Response req 14 2021-08-27 11:01:34.604814 +0800 CST m=+6.000242227
Response req 15 2021-08-27 11:01:35.604779 +0800 CST m=+7.000214163
Response req 16 2021-08-27 11:01:35.604818 +0800 CST m=+7.000252973
Response req 17 2021-08-27 11:01:36.60477 +0800 CST m=+8.000211783
Response req 18 2021-08-27 11:01:36.604802 +0800 CST m=+8.000244230
Response req 19 2021-08-27 11:01:37.604764 +0800 CST m=+9.000213264
Response req 20 2021-08-27 11:01:37.604834 +0800 CST m=+9.000282691
```
可以看到20个请求都处理了，但是还是限制了每秒只有2个请求。

计数器算法存在**时间临界点**缺陷。比如每一分钟限速100个请求（也就是每秒最多1.7个请求），在00:00:00到00:00:58这段时间内没有任何用户请求，然后在00:00:59这一瞬时发出了100个请求，这是允许的，然后在
00:01:00这一瞬时又发出了100个请求，短短1s内发出了200个请求，系统可能会承受恶意用户的大量请求，甚至击穿系统。


### 2.滑动窗口(TODO)
针对计数器存在的临界点缺陷
>滑动窗口把固定时间片进行划分，并且随着时间的流逝，进行移动，固定数量的可以移动的格子，进行计数并判断阈值


### 3.漏桶算法
漏桶算法描述如下:
- 一个固定容量的漏桶，按照固定速率流出水滴
- 如果桶是空的，则不需要流出水滴
- 可以以任意速率流入水滴到漏桶
- 如果流入水滴超过了桶的容量，则溢出（被丢弃）

![leaky-bucket.png](./image/leaky-bucket.png)

漏桶算法思路很简单，水（请求）先进入漏桶里，漏桶以一定的速度出水，当水流速度过大会直接溢出，可以看出漏桶算法能强行限制数据的传输速率。通俗点来说，我们有一个固定容量的桶，有水流进来，也有水流出去。对于
流进来的水（请求）来说，我们无法预计一共有多少水会流进来，也无法预计水流的速度。但是对于流出去的水来说，这个桶可以固定水流出的速率（处理速度），从而达到**流量整形(Traffic Shaping)**和**流量控制(Traffic Policing)**的效果。
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

func (lb *LeakyBucket) Create(rate float64, capacity float64) {
	lb.rate = rate
	lb.capacity = capacity
	lb.water = 0
	lb.lastLeakyMs = time.Now().UnixNano() / 1e6

	//(2)异步执行漏水的操作
	go func() {
		for {
			lb.lock.Lock()
			now := time.Now().UnixNano() / 1e6 //ms
			expand := float64(now - lb.lastLeakyMs) / 1000 *  lb.rate
			//(2.1)计算剩余水量,需要判断桶是否已经干了
			lb.water = lb.water - expand
			lb.water = math.Max(0, lb.water)
			lb.lastLeakyMs = now
			lb.lock.Unlock()
			//(2.2)sleep一段时间再执行漏水操作
			time.Sleep(time.Duration(1e3/rate) * time.Millisecond)
		}
	}()
}

func (lb *LeakyBucket) Allow() bool {
	lb.lock.Lock()
	defer lb.lock.Unlock()

	// 如果新加的水量之后桶中当前水量大于桶的容量表示水满执行溢出操作
	if lb.water+1 > lb.capacity {
		return false
	} else {
		lb.water++
		return true
	}
}

func main() {
	var leaky LeakyBucket
	leaky.Create(3, 3)

	wg := sync.WaitGroup{}

	for i := 1; i <= 40; i++ {
		wg.Add(1)

		go func(i int) {
			if leaky.Allow() {
				fmt.Println("Response req", i, time.Now())
			}
			wg.Done()
		}(i)
		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
}
```
这里初始化桶容量为3，桶内无水，每隔0.1s的形式创建40个请求，随着请求往里面"滴水"，桶按照每秒3个单位的速率流出水，处理时，部分请求被丢弃：
```shell
Response req 1 2021-08-27 11:27:35.490245 +0800 CST m=+0.000148829
Response req 2 2021-08-27 11:27:35.593785 +0800 CST m=+0.103690665
Response req 3 2021-08-27 11:27:35.698835 +0800 CST m=+0.208742557
Response req 5 2021-08-27 11:27:35.905212 +0800 CST m=+0.415122197
Response req 8 2021-08-27 11:27:36.211181 +0800 CST m=+0.721094780
Response req 11 2021-08-27 11:27:36.523751 +0800 CST m=+1.033668336
Response req 15 2021-08-27 11:27:36.937492 +0800 CST m=+1.447414567
Response req 18 2021-08-27 11:27:37.244769 +0800 CST m=+1.754694524
Response req 21 2021-08-27 11:27:37.558478 +0800 CST m=+2.068407889
Response req 24 2021-08-27 11:27:37.868595 +0800 CST m=+2.378527728
Response req 27 2021-08-27 11:27:38.182616 +0800 CST m=+2.692552789
Response req 31 2021-08-27 11:27:38.593027 +0800 CST m=+3.102968021
Response req 34 2021-08-27 11:27:38.902344 +0800 CST m=+3.412288548
Response req 37 2021-08-27 11:27:39.207447 +0800 CST m=+3.717395642
```


### 4.令牌桶算法
由于漏桶出水速度时恒定的，如果瞬时爆发大流量的话，将有大部分请求被丢弃掉（溢出）。为了解决这个问题，产生了令牌桶算法。令牌桶算法描述如下：
- 有一个固定容量的桶，桶一开始是空的
- 以固定的速率**r**往tong桶里填充**token**，直到达到桶的容量，多余的令牌将会丢弃
- 每当一个请求过来时，就尝试从桶里移除一个令牌，如果没有令牌的话，请求无法通过

![token-bucket.jpg](./image/token-bucket.jpg)

```golang
/**
令牌桶算法的原理是系统会以一个恒定的速度往桶里放入令牌，而如果请求需要处理，则需要先从桶里获取一个令牌，当桶里没有令牌可取时，则拒绝服务
*/
package main

import (
	"fmt"
	"math"
	"sync"
	"time"
)

type TokenBucket struct {
	rate         int64   //固定的token放入速率 r/s
	capacity     float64 //桶的容量
	tokens       float64 //桶中当前token数量
	lastTokenMs int64   //桶上次放token的时间戳 ms

	lock sync.Mutex
}

func (token *TokenBucket) Create(rate int64, capacity float64) {
	token.rate = rate
	token.capacity = capacity
	token.lastTokenMs = time.Now().UnixNano() / 1e6

	//(1)初始化桶中token数量为桶的容量
	token.tokens = capacity

	//(2)异步往桶中添加令牌，令牌的数量最大值是桶的容量
	go func() {
		for {
			token.lock.Lock()
			now := time.Now().UnixNano() / 1e6
			//(2.1)先往桶中添加令牌
			token.tokens = token.tokens + float64((now-token.lastTokenMs)*token.rate)/1e3
			token.tokens = math.Min(token.capacity, token.tokens)
			token.lastTokenMs = now
			token.lock.Unlock()
			//(2.2)sleep一段时间再执行添加令牌的操作
			time.Sleep(time.Duration(1e3/rate) * time.Millisecond)
		}
	}()
}

func (token *TokenBucket) Allow() bool {
	if token.tokens-1 < 0 {
		return false
	} else {
		token.tokens--
		return true
	}
}

func main() {
	var wg sync.WaitGroup
	var lr TokenBucket
	lr.Create(3, 5) //每秒访问速率限制为3个请求，桶容量为3

	for i := 1; i <= 40; i++ {
		wg.Add(1)

		go func(i int) {
			if lr.Allow() {
				fmt.Println("Response req", i, time.Now())
			}
			wg.Done()
		}(i)

		time.Sleep(100 * time.Millisecond)
	}
	wg.Wait()
}

```
这里初始化桶容量为5个单位，桶中有5个令牌。之后每1s产生3个令牌。而后每隔0.1s的方式创建40个请求，获取访问令牌:
```shell
Response req 1 2021-08-27 11:32:41.562041 +0800 CST m=+0.000127316
Response req 2 2021-08-27 11:32:41.664894 +0800 CST m=+0.102981922
Response req 3 2021-08-27 11:32:41.767195 +0800 CST m=+0.205283293
Response req 4 2021-08-27 11:32:41.872047 +0800 CST m=+0.310136350
Response req 5 2021-08-27 11:32:41.976412 +0800 CST m=+0.414501683
Response req 6 2021-08-27 11:32:42.080658 +0800 CST m=+0.518748402
Response req 8 2021-08-27 11:32:42.287345 +0800 CST m=+0.725436938
Response req 11 2021-08-27 11:32:42.600802 +0800 CST m=+1.038896037
Response req 14 2021-08-27 11:32:42.910744 +0800 CST m=+1.348840433
Response req 18 2021-08-27 11:32:43.324245 +0800 CST m=+1.762343491
Response req 21 2021-08-27 11:32:43.631339 +0800 CST m=+2.069440057
Response req 24 2021-08-27 11:32:43.943049 +0800 CST m=+2.381152027
Response req 27 2021-08-27 11:32:44.250166 +0800 CST m=+2.688271272
Response req 31 2021-08-27 11:32:44.662027 +0800 CST m=+3.100135390
Response req 34 2021-08-27 11:32:44.974085 +0800 CST m=+3.412194673
Response req 37 2021-08-27 11:32:45.28616 +0800 CST m=+3.724272456
Response req 40 2021-08-27 11:32:45.599233 +0800 CST m=+4.037347100
```
由于桶中可以储备令牌，这使得令牌桶算法支持一定程度突发的大流量并发访问，也就是说，假设桶内有100个token时，那么可以瞬间允许100个请求通过。


### 漏桶算法和令牌桶算法对比
- 令牌桶是按照固定速率往桶中添加令牌，请求是否被处理需要看桶中令牌是否足够，当令牌数减为0时则拒绝新的请求
- 漏桶则是按照常量固定速率流出请求，流入请求速率任意，当流入的请求数累计到漏桶容量时，则新流入的请求被拒绝
- 令牌桶限制的是平均流入速率（允许突发请求，只要有令牌就可以处理，支持一次拿3个令牌，4个令牌），并允许一定程度的突发流量
- 漏桶限制的是常量流出速率（即流出速率是一个固定常量值，比如都是1的速率流出，而不能一次是1，下次是2），从而平滑突发流入速率
- 令牌桶允许一定程度的突发，而漏桶主要目的是平滑流入速率
- 两个算法实现可以一样，但是方向是相反的，对于相同的参数得到的限流效果是一样的


### 5.golang原生rate包
Go语言中的**golang.org/x/time/rate**包采用了令牌桶算法来实现速率限制。

#### rate包的简单介绍

#### 用法示例
```golang
package main

import (
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

var (
	limiter = rate.NewLimiter(2, 5)
)

func limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, http.StatusText(http.StatusTooManyRequests)+"; time="+time.Now().Format("2006-01-02 15:04:05.000"), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("OK; time=" + time.Now().Format("2006-01-02 15:04:05.000") + "\n"))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", okHandler)
	_ = http.ListenAndServe(":3000", limit(mux))
}

```
测试结果
```
while true; do  curl http://localhost:3000/; sleep 0.1s; done;

OK; time=2021-08-27 11:39:40.465
OK; time=2021-08-27 11:39:40.591
OK; time=2021-08-27 11:39:40.716
OK; time=2021-08-27 11:39:40.842
OK; time=2021-08-27 11:39:40.959
OK; time=2021-08-27 11:39:41.081
Too Many Requests; time=2021-08-27 11:39:41.205
Too Many Requests; time=2021-08-27 11:39:41.328
Too Many Requests; time=2021-08-27 11:39:41.448
OK; time=2021-08-27 11:39:41.570
Too Many Requests; time=2021-08-27 11:39:41.692
Too Many Requests; time=2021-08-27 11:39:41.817
Too Many Requests; time=2021-08-27 11:39:41.942
OK; time=2021-08-27 11:39:42.066
Too Many Requests; time=2021-08-27 11:39:42.192
Too Many Requests; time=2021-08-27 11:39:42.310
Too Many Requests; time=2021-08-27 11:39:42.431
OK; time=2021-08-27 11:39:42.548
Too Many Requests; time=2021-08-27 11:39:42.674
Too Many Requests; time=2021-08-27 11:39:42.799
Too Many Requests; time=2021-08-27 11:39:42.921
OK; time=2021-08-27 11:39:43.038
Too Many Requests; time=2021-08-27 11:39:43.154
Too Many Requests; time=2021-08-27 11:39:43.269
Too Many Requests; time=2021-08-27 11:39:43.391
OK; time=2021-08-27 11:39:43.513
Too Many Requests; time=2021-08-27 11:39:43.636
Too Many Requests; time=2021-08-27 11:39:43.757
Too Many Requests; time=2021-08-27 11:39:43.880
OK; time=2021-08-27 11:39:43.997
Too Many Requests; time=2021-08-27 11:39:44.122
Too Many Requests; time=2021-08-27 11:39:44.244
Too Many Requests; time=2021-08-27 11:39:44.363

```
后续的请求每秒中只有2个请求让通过


### 参考文献
- https://hxzqlh.com/2018/09/12/go-rate-limit/