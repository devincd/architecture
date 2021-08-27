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
