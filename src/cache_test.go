package src

import (
	"testing"
	"log"
	"os"
	"time"
	"strconv"
)

var (
	k = "testkey"
	v = "testvalue"
)

type myStruct struct {
	text     string
	moreData []byte
}

func TestAdd(t *testing.T) {
	cache := Cache("myCache")
	cache.SetLogger(log.New(os.Stderr, "", log.LstdFlags))

	cache.SetAddedItemCallback(func(item *CacheItem) {
		cache.items[item.key] = item
	})

	m := myStruct{"This is a test", []byte{}}
	cache.Add("someKey", 20*time.Second, &m)

	c, e := cache.Value("someKey")
	if e != nil {
		cache.log(e.Error())
		return
	}
	cache.log("key:", c.key, "value:", c.data)

	time.Sleep(1000 * time.Second)
}

func TestFlush(t *testing.T) {
	cache := Cache("myCacheFlush")
	cache.SetLogger(log.New(os.Stderr, "", log.LstdFlags))

	cache.SetAddedItemCallback(func(item *CacheItem) {
		cache.items[item.key] = item
	})
	cache.Add(k, 100*time.Second, v)

	cache.Flush()

	p, err := cache.Value(k)

	if os.IsExist(err) || p != nil {
		cache.log(err.Error(), p)
		return
	}
	if cache.Count() != 0 {
		cache.log("Error verfying count of flushed table")
	}
	time.Sleep(1000 * time.Second)
}

func TestCount(t *testing.T) {

	cache := Cache("testCount")

	count := 10000

	for i := 0; i < count; i++ {
		key := k + strconv.Itoa(i)
		cache.Add(key, 20*time.Second, v)
	}

}
