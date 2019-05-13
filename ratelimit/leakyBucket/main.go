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
