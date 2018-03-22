package src

import (
	"sync"
	"time"
	"log"
	"sort"
)

type CacheTable struct {
	sync.RWMutex
	name  string
	items map[interface{}]*CacheItem
	//定时器负责触发清理。
	cleanupTimer *time.Timer
	//当前计时器持续
	cleanupInterval   time.Duration
	logger            *log.Logger
	loadData          func(key interface{}, args ...interface{}) *CacheItem
	addedItem         func(item *CacheItem)
	aboutToDeleteItem func(item *CacheItem)
}

//计数返回当前存储在缓存中的项目数。
func (table *CacheTable) Count() int {
	table.Lock()
	defer table.Unlock()
	return len(table.items)
}

//遍历 所有的items
func (table *CacheTable) Foreach(trans func(key interface{}, item *CacheItem)) {
	table.Lock()
	defer table.Unlock()

	for k, v := range table.items {
		trans(k, v)
	}
}

// SetAddedItemCallback configures a callback, which will be called every time
// a new item is added to the cache.
func (table *CacheTable) SetAddedItemCallback(f func(*CacheItem)) {
	table.Lock()
	defer table.Unlock()
	table.addedItem = f
}

//SetLogger
func (table *CacheTable) SetLogger(logger *log.Logger) {
	table.Lock()
	defer table.Unlock()
	table.logger = logger
}

//print logger
func (table *CacheTable) log(v ...interface{}) {
	if table.logger == nil {
		return
	}
	table.logger.Println(v...)
}

//过期检查循环，由自我调整的计时器触发。
func (table *CacheTable) expirationCheck() {

	table.Lock()

	if table.cleanupTimer != nil {
		table.cleanupTimer.Stop()
	}
	if table.cleanupInterval > 0 {
		table.log("Expiration check triggered after", table.cleanupInterval, "for table", table.name)
	} else {
		table.log("Expiration check installed for table", table.name)
	}

	now := time.Now()

	smallesDuration := 0 * time.Second

	for key, item := range table.items {
		item.Lock()

		lifeSpan := item.lifeSpan
		createdOn := item.createdOn
		item.Unlock()

		if lifeSpan == 0 {
			continue
		}

		if now.Sub(createdOn) >= lifeSpan {
			//物品已过期其寿命。
			table.Delete(key)
		} else {
			//按照时间顺序查找最接近寿命结束的项目。

			if smallesDuration == 0 || lifeSpan-now.Sub(createdOn) < smallesDuration {
				smallesDuration = lifeSpan - now.Sub(createdOn)
			}
		}
	}

	table.cleanupInterval = smallesDuration
	if smallesDuration > 0 {
		//多少时间以后执行方法
		//于f函数不是在Main Line执行的，而是注册在goroutine Line里执行的
		//所以一旦后悔的话，需要使用Stop命令来停止即将开始的执行，如果已经开始执行就来不及了
		table.cleanupTimer = time.AfterFunc(smallesDuration, func() {
			go table.expirationCheck()
		})
	}
	table.Unlock()

}

func (table *CacheTable) addInternal(item *CacheItem) {
	table.log("Adding item with key:", item.key, "add lifeSpan of ", item.lifeSpan, "to table", table.name)
	table.items[item.key] = item

	exDur := table.cleanupInterval
	addedItem := table.addedItem

	table.Unlock()

	if addedItem != nil {
		addedItem(item)
	}

	if item.lifeSpan > 0 && (exDur == 0 || item.lifeSpan < exDur) {
		table.expirationCheck()
	}
}

func (table *CacheTable) Add(key interface{}, lifeSpan time.Duration, data interface{}) *CacheItem {

	item := NewCacheItem(key, lifeSpan, data)
	table.Lock()
	table.addInternal(item)

	return item

}

func (table *CacheTable) deleteInternal(key interface{}) (*CacheItem, error) {
	r, ok := table.items[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	aboutToDeleteItem := table.aboutToDeleteItem

	table.Unlock()
	if aboutToDeleteItem != nil {
		aboutToDeleteItem(r)
	}

	r.RLock()
	if r.aboutToExpire != nil {
		r.aboutToExpire(key)
	}
	table.Lock()
	table.log("Deleting item with key", key, "created on", r.createdOn, "and hit", r.accessCount, "times from table", table.name)

	delete(table.items, key)
	return r, nil
}

func (table *CacheTable) Delete(key interface{}) (*CacheItem, error) {
	table.Lock()
	defer table.Unlock()

	return table.deleteInternal(key)
}

func (table *CacheTable) Exists(key interface{}) bool {
	table.RLock()
	defer table.RUnlock()
	//判断key存不存在
	_, ok := table.items[key]
	return ok
}

//// NotFoundAdd测试是否找不到缓存中的项目。 与Exists不同
//方法这也会添加数据，如果他们的钥匙找不到。
func (table *CacheTable) NotFoundAdd(key interface{}, lifeSpan time.Duration, data interface{}) bool {
	table.Lock()
	if _, ok := table.items[key]; ok {
		table.Unlock()
		return false
	}

	item := NewCacheItem(key, lifeSpan, data)
	table.addedItem(item)
	return true
}

func (table *CacheTable) Value(key interface{}, args ...interface{}) (*CacheItem, error) {

	table.RLock()

	r, ok := table.items[key]
	loadData := table.loadData
	table.RUnlock()

	if ok {
		r.KeepAlive()
		return r, nil
	}

	if loadData != nil {
		item := loadData(key, args)

		if item != nil {
			table.Add(key, item.lifeSpan, item.data)
			return item, nil
		}
		return nil, ErrKeyNotFoundOrLoadable
	}
	return nil, ErrKeyNotFound

}

func (table *CacheTable) Flush() {

	table.Lock()

	defer table.Unlock()

	table.log("Flushing table", table.name)
	//重置CacheItem
	table.items = make(map[interface{}]*CacheItem)

	table.cleanupInterval = 0

	if table.cleanupTimer != nil {
		//Stop停止所有在time条件下等待执行的方法
		table.cleanupTimer.Stop()
	}
}

//CacheItemPair将密钥映射到访问计数器
type CacheItemPair struct {
	key         interface{}
	AccessCount int64
}

type CacheItemPairList []CacheItemPair

func (p CacheItemPairList) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p CacheItemPairList) Len() int {
	return len(p)
}
func (p CacheItemPairList) Less(i, j int) bool {
	return p[i].AccessCount > p[j].AccessCount
}

func (table *CacheTable) MostAccessed(count int64) []*CacheItem {
	table.Lock()
	defer table.Unlock()

	p := make(CacheItemPairList, len(table.items))
	i := 0

	for k, v := range table.items {
		p[i] = CacheItemPair{k, v.accessCount}
		i++
	}
	sort.Sort(p)

	var r []*CacheItem

	c := int64(0)

	for _, v := range p {
		if c >= count {
			break
		}
		item, ok := table.items[v.key]
		if ok {
			r = append(r, item)
		}
		c++
	}
	return r
}
