package src

import (
	"sync"
	"time"
)

type CacheItem struct {
	//读写锁
	sync.RWMutex
	//cache key
	key interface{}
	//cache value
	data interface{}
	//在不被访问/保持活动状态时，该项目在缓存中存活多久。
	lifeSpan time.Duration
	//创建时间戳。
	createdOn time.Time
	//上次访问时间戳。
	accessedON time.Time
	//该项目被访问的频率。
	accessCount int64
	//在从缓存中删除项目之前触发的回调方法
	aboutToExpire func(key interface{})
}

// NewCacheItem返回一个新创建的CacheItem。
//参数键是项目的缓存键。
//参数lifeSpan确定在没有访问该项目的时间段之后
//将从缓存中移除。
//参数数据是项目的值。
func NewCacheItem(key interface{}, lifeSpan time.Duration, data interface{}) *CacheItem {
	t := time.Now()
	return &CacheItem{
		key:           key,
		data:          data,
		lifeSpan:      lifeSpan,
		createdOn:     t,
		accessedON:    t,
		aboutToExpire: nil,
		accessCount:   0,
	}
}

//KeepAlive标记一个项目要保留另一个expireDuration周期。
func (item *CacheItem) KeepAlive() {
	item.Lock()
	defer item.Unlock()

	item.accessedON = time.Now()
	item.accessCount++
}

//返回过期的时间
func (item *CacheItem) LifeSpan() time.Duration {
	return item.lifeSpan
}

//获取上次访问时间戳。
func (item *CacheItem) AccessedOn() time.Time {
	return item.accessedON
}

//获取该项目被访问的频率。
func (item *CacheItem) AccessCount() int64 {
	return item.accessCount
}

func (item *CacheItem) Key() interface{} {
	return item.key
}
func (item *CacheItem) Data() interface{} {
	return item.data
}

//set在从缓存中删除项目之前触发的回调方法
func (item *CacheItem) SetAboutToExpireCallback(fn func(key interface{})) {
	item.Lock()
	defer item.Unlock()
	item.aboutToExpire = fn
}

