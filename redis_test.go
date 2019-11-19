package zlimiter_test

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/zzerroo/zlimiter"
	rds "github.com/zzerroo/zlimiter/driver/redis"
)

func TestRedisFixWindow(t *testing.T) {

	key := "test"
	redisLimit, erro := zlimiter.NewLimiter(zlimiter.LimitRedisFixWindow, rds.RedisInfo{Address: "127.0.0.1:6379", Passwd: "test"})
	if erro != nil {
		t.Error(erro.Error())
	}

	// Test Add
	erro = redisLimit.Add(key, 10, 2*time.Second)
	if erro != nil {
		t.Error(erro.Error())
	}

	// Test Get
	bReached, left, erro := redisLimit.Get(key)
	if bReached != false || left != 9 || erro != nil {
		t.Error(bReached, left, erro)
	}

	// Test timeout
	time.Sleep(3 * time.Second)
	bReached, left, erro = redisLimit.Get(key)
	if bReached != false || left != 9 || erro != nil {
		t.Error(bReached, left, erro)
	}

	// Test Set
	erro = redisLimit.Set(key, 15, 4*time.Second)
	if erro != nil {
		t.Error(erro.Error())
	}

	bReached, left, erro = redisLimit.Get(key)
	if bReached != false || left != 14 || erro != nil {
		t.Error(bReached, left, erro)
	}

	// Test Sync Get
	var successCnt, failCnt int
	var wg sync.WaitGroup
	for i := 0; i < 18; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			bReached, left, erro := redisLimit.Get(key)
			if bReached != false || left < 0 || erro != nil {
				failCnt++
			} else {
				successCnt++
			}
		}(i)
	}

	wg.Wait()
	if failCnt != 4 {
		t.Error(failCnt)
	}

	// Test Del
	erro = redisLimit.Del(key)
	if erro != nil {
		t.Error(erro.Error())
	}

	bReached, left, erro = redisLimit.Get(key)
	if bReached != true || left != -2 || erro != nil {
		t.Error(bReached, left, erro)
	}
}

func TestRedisSlideWindow(t *testing.T) {

	key := "test"
	redisLimit, erro := zlimiter.NewLimiter(zlimiter.LimitRedisSlideWindow, rds.RedisInfo{Address: "127.0.0.1:6379", Passwd: "test"})
	if erro != nil {
		t.Error(erro.Error())
	}

	// Test Add
	erro = redisLimit.Add(key, 10, 2*time.Second)
	if erro != nil {
		t.Error(erro.Error())
	}

	// Test Get
	bReached, left, erro := redisLimit.Get(key)
	if bReached != false || left != 9 || erro != nil {
		t.Error(bReached, left, erro)
	}

	// Test timeout
	time.Sleep(3 * time.Second)
	bReached, left, erro = redisLimit.Get(key)
	if bReached != false || left != 9 || erro != nil {
		t.Error(bReached, left, erro)
	}

	// Test Set
	erro = redisLimit.Set(key, 15, 4*time.Second)
	if erro != nil {
		t.Error(erro.Error())
	}

	bReached, left, erro = redisLimit.Get(key)
	if bReached != false || left != 14 || erro != nil {
		t.Error(bReached, left, erro)
	}

	// Test Sync Get
	var successCnt, failCnt int
	var wg sync.WaitGroup
	for i := 0; i < 18; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			bReached, left, erro := redisLimit.Get(key)
			if bReached != false || left < 0 || erro != nil {
				failCnt++
			} else {
				successCnt++
			}
		}(i)
	}

	wg.Wait()
	if failCnt != 4 {
		t.Error(failCnt)
	}

	// test overflow
	erro = redisLimit.Set(key, 15, 4*time.Second)
	if erro != nil {
		t.Error(erro.Error())
	}

	time.Sleep(1 * time.Second)
	bReached, left, erro = redisLimit.Get(key)
	if bReached != false || left != 14 || erro != nil {
		t.Error(bReached, left, erro)
	}

	time.Sleep(1 * time.Second)
	bReached, left, erro = redisLimit.Get(key)
	if bReached != false || left != 13 || erro != nil {
		t.Error(bReached, left, erro)
	}

	time.Sleep(1 * time.Second)
	bReached, left, erro = redisLimit.Get(key)
	if bReached != false || left != 12 || erro != nil {
		t.Error(bReached, left, erro)
	}

	time.Sleep(2 * time.Second)
	bReached, left, erro = redisLimit.Get(key)
	if bReached != false || left != 12 || erro != nil {
		t.Error(bReached, left, erro)
	}

	// Test Del
	erro = redisLimit.Del(key)
	if erro != nil {
		t.Error(erro.Error())
	}

	bReached, left, erro = redisLimit.Get(key)
	if bReached != true || left != -2 || erro != nil {
		t.Error(bReached, left, erro)
	}
}

func TestRedisToken(t *testing.T) {
	key := "test"
	var reached bool = false
	var left, max int64 = 0, 20

	// create
	redisLimit, erro := zlimiter.NewLimiter(zlimiter.LimitRedisToken, rds.RedisInfo{Address: "127.0.0.1:6379", Passwd: "test"})
	if erro != nil {
		t.Errorf("error:%s", erro.Error())
	}

	// test add
	erro = redisLimit.Add(key, 4, 4*time.Second, max)
	if erro != nil {
		t.Errorf("error:%s", erro.Error())
	}

	time.Sleep(1 * time.Second)

	// reached,left == false,0
	reached, left, erro = redisLimit.Get(key)
	if reached == true || left != 0 || erro != nil {
		t.Errorf("%v,%v,%v,should be false, 0, nil", reached, left, erro)
	}
	

	// reached,left == true,-1
	reached, left, erro = redisLimit.Get(key)
	if erro != nil || reached == false {
		t.Errorf("%v,%v,%v,should be false, 0, nil", reached, left, erro)
	}

	// create 4 token
	time.Sleep(4 * time.Second)

	// test get and limit
	sCnt := 0
	fCnt := 0
	var wg sync.WaitGroup
	for i := 0; i < 14; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reach, _, erro := redisLimit.Get(key)
			if erro == nil && reach == false {
				sCnt++
			} else {
				fCnt++
				if erro != nil {
					t.Logf("error:%s", erro.Error())
				}
			}
		}()
	}

	wg.Wait()

	//sCnt == 4
	if sCnt != 4 {
		t.Errorf("sCnt %d,should be 4", sCnt)
	}

	// test set
	erro = redisLimit.Set(key, 4, 2*time.Second, max)
	if erro != nil {
		t.Errorf("error %s", erro.Error())
	}

	time.Sleep(4 * time.Second)

	// reached,left == false,7
	reached, left, erro = redisLimit.Get(key)
	if reached != false || left != 7 {
		t.Errorf("%v,%v,%v,should be false, 7, nil", reached, left, erro)
	}

	// test overflow
	erro = redisLimit.Set(key, 4, 4*time.Second, max)
	if erro != nil {
		t.Errorf("error %s", erro.Error())
	}

	time.Sleep(1 * time.Second)

	// reached,left == false 0
	reached, left, erro = redisLimit.Get(key)
	if reached != false || left != 0 {
		t.Errorf("%v,%v,%v,should be false, 0, nil", reached, left, erro)
	}

	time.Sleep(25 * time.Second)

	// reached,left == false 19
	reached, left, erro = redisLimit.Get(key)
	if reached != false || left != 19 {
		t.Errorf("%v,%v,%v,should be false, 19, nil", reached, left, erro)
	}

	// reached,left == false 18
	reached, left, erro = redisLimit.Get(key)
	if reached != false || left != 18 {
		t.Errorf("%v,%v,%v,should be false, 18, nil", reached, left, erro)
	}

	// test del
	redisLimit.Del(key)
	_, _, erro = redisLimit.Get(key)
	if erro == nil {
		t.Error("should not find the key")
	}
}

func TestRedisBucket(t *testing.T) {
	// test add
	key := "test"
	reached := false
	var left, max, sCnt, fCnt int64 = 0, 20, 0, 0

	redisLimit, erro := zlimiter.NewLimiter(zlimiter.LimitRedisBucket, rds.RedisInfo{Address: "127.0.0.1:6379", Passwd: "test"})
	if erro != nil {
		t.Errorf("error:%s", erro.Error())
	}

	erro = redisLimit.Add(key, 4, 4*time.Second, max)
	if erro != nil {
		t.Errorf("error:%s", erro.Error())
	}

	// reached，left == false,-1
	tm1 := time.Now()
	reached, left, erro = redisLimit.Get(key)
	if erro != nil || reached != false || left != -1 {
		t.Errorf("%v,%v,%v,should false,-1,nil", reached, left, erro)
	}

	// reached,left == true,0
	reached, left, erro = redisLimit.Get(key)
	if erro != nil || reached != false || left != -1 {
		t.Errorf("%v,%v,%v,should true,0,nil", reached, left, erro)
	}
	tm2 := time.Now()

	// duration about 1s
	duSec := tm2.Sub(tm1).Seconds()
	if int64(duSec) != 1 {
		t.Errorf("%v,tm duration should be 1 sec", int64(duSec))
	}

	// test get and limit
	var wg sync.WaitGroup
	for i := 0; i < 14; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reach, _, erro := redisLimit.Get(key)
			if erro == nil && reach == false && left == -1 {
				sCnt++
			} else {
				fmt.Printf("11 %v\n", erro)
				fCnt++
			}
		}()
	}

	wg.Wait()
	if sCnt != 14 {
		t.Errorf("%v,sCnt should be 13", sCnt)
	}

	// test set
	erro = redisLimit.Set(key, 4, 8*time.Second, max)
	if erro != nil {
		t.Errorf("error %s", erro.Error())
	}

	// reached,left == false,-1
	tm1 = time.Now()
	reached, left, erro = redisLimit.Get(key)
	if reached != false || left != -1 || erro != nil {
		t.Errorf("%v,%v,%v,should false,-1,nil", reached, left, erro)
	}

	// reached,left == false,-1
	reached, left, erro = redisLimit.Get(key)
	if reached != false || left != -1 || erro != nil {
		t.Errorf("%v,%v,%v,should false,-1,nil", reached, left, erro)
	}
	tm2 = time.Now()

	duSec = tm2.Sub(tm1).Seconds()
	if int64(duSec) != 2 {
		t.Errorf("%v, int64(duSec) should be 2 sec", int64(duSec))
	}

	erro = redisLimit.Set(key, 4, 8*time.Second, max)
	if erro != nil {
		t.Errorf("error %s", erro.Error())
	}

	//reached,left == false 0
	tm1 = time.Now()
	reached, left, erro = redisLimit.Get(key)
	if reached != false || left != -1 || erro != nil {
		t.Errorf("%v,%v,%v,should false,-1,nil", reached, left, erro)
	}

	time.Sleep(1500 * time.Millisecond)

	// reached,left == false,0
	reached, left, erro = redisLimit.Get(key)
	if reached != false || left != -1 || erro != nil {
		t.Errorf("%v,%v,%v,should false,0,nil", reached, left, erro)
	}

	tm2 = time.Now()
	duMs := tm2.Sub(tm1).Nanoseconds() / 1e6
	if math.Abs(float64(int64(duMs)-2000)) >= 100 {
		t.Errorf("%v,math.Abs(float64(int64(duMs)-2000)) >= 100", math.Abs(float64(int64(duMs)-2000)))
	}

	// test del
	redisLimit.Del(key)
	_, _, erro = redisLimit.Get(key)
	if erro == nil {
		t.Error("should not find the key")
	}
}
