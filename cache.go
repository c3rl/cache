package c3rlcache

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type cacheEntry struct {
	added          uint64
	expireAfter    uint64
	expirecount    uint64
	dataB          []byte
	data           string
	hit            uint64
	deleteOnExpire bool
}

type CacheClient struct {
	totalEntries uint64
	maxEntries   uint64
	entries      map[string]*cacheEntry
	// expire       uint64
	expirecount uint64
	pushOnFull  bool
}

type CacheClientConfig struct {
	TotalEntries uint64
	MaxEntries   uint64
	// Expire       uint64
	ExpireCount uint64
	PushOnFull  bool
}

type c3rlGeneralPayloadV2 struct {
	Payload interface{} `json:"payload" validate:"required"`
	Status  string      `json:"status" validate:"required"`
	Code    int         `json:"code" validate:"required"`
}

func NewCacheClient(conf CacheClientConfig) CacheClient {

	var client CacheClient

	client = CacheClient{
		totalEntries: conf.TotalEntries,
		maxEntries:   conf.MaxEntries,
		// expire:       conf.Expire,
		expirecount: conf.ExpireCount,
		pushOnFull:  client.pushOnFull,
	}

	if conf.TotalEntries > 0 {
		client.entries = make(map[string]*cacheEntry, conf.TotalEntries)
	} else {
		client.entries = make(map[string]*cacheEntry)
	}

	return client

}

func (c *CacheClient) AddByte(route string, data []byte, expireAfter uint64, expireCount uint64, deleteOnExpire bool) error {

	if uint64(len(c.entries)) == c.maxEntries {

		if c.pushOnFull {
			/* delete oldest */
		}

		return fmt.Errorf("no space left")
	}

	curr_time := uint64(time.Now().Unix())

	c.entries[route] = &cacheEntry{
		expireAfter:    expireAfter,
		added:          curr_time,
		expirecount:    expireCount,
		dataB:          data,
		deleteOnExpire: deleteOnExpire,
	}

	return nil
}

func (c *CacheClient) Add(route string, data string, expireAfter uint64, expireCount uint64, deleteOnExpire bool) error {

	if uint64(len(c.entries)) == c.maxEntries {
		return fmt.Errorf("no space left")
	}

	curr_time := uint64(time.Now().Unix())

	c.entries[route] = &cacheEntry{
		expireAfter:    expireAfter,
		added:          curr_time,
		expirecount:    expireCount,
		data:           data,
		deleteOnExpire: deleteOnExpire,
	}

	return nil
}

func (c *CacheClient) GetB(route string) ([]byte, error) {
	data, ok := c.entries[route]
	if !ok {
		return []byte{}, fmt.Errorf("route does not exist")
	}

	c.entries[route].hit += 1

	return data.dataB, nil
}

func (c *CacheClient) Get(route string, repopulate bool) (string, error) {

	data, ok := c.entries[route]
	if !ok {
		return "", fmt.Errorf("route does not exist")
	}

	// check if expired

	curr_time := uint64(time.Now().Unix())

	if curr_time > data.expireAfter+data.added {
		if !repopulate || data.deleteOnExpire {
			if data.deleteOnExpire {
				c.Delete(route)
			}
			return "", fmt.Errorf("expired")
		}
		c.entries[route].added = curr_time
	}

	if data.expirecount > 0 {
		if data.expirecount < data.hit {
			if !repopulate || data.deleteOnExpire {
				if data.deleteOnExpire {
					c.Delete(route)
				}
				return "", fmt.Errorf("count expired")
			}
			c.entries[route].hit = 0
		}
	}

	c.entries[route].hit += 1

	return data.data, nil
}

func (c *CacheClient) Delete(route string) error {

	delete(c.entries, route)

	return nil
}

func (c *CacheClient) IsCached(route string) bool {

	_, ok := c.entries[route]
	if !ok {
		return false
	}
	return true
}

func (c *CacheClient) RouteMiddleware(w http.ResponseWriter, r *http.Request, expireAfter uint64, expireCount uint64, deleteOnExpire bool, function_to_execute func() (interface{}, error), function_to_write func(interface{}) error,

// headers map[string][]string
) (err error) {

	var data_to_send interface{}

CACHEDATA:
	if c.IsCached(r.RequestURI) {
		data, err := c.Get(r.RequestURI, false)
		if err != nil {
			if !deleteOnExpire {
				c.Delete(r.RequestURI)
			}
			goto CACHEDATA /* TODO: make sure it doesnt recheck c.IsCached */
		}
		json.Unmarshal([]byte(data), &data_to_send)

	} else {
		data_to_send, err = function_to_execute()
		if err != nil {
			return err
		}
		data_to_add, err := json.Marshal(data_to_send)
		if err != nil {
			return err
		}
		err = c.Add(r.RequestURI, string(data_to_add), expireAfter, expireCount, deleteOnExpire)
		if err != nil {
			return err
		}
	}

	/* set headers */
	// for key, vals := range headers {
	// 	for _, val := range vals {
	// 		w.Header().Add(key, val)
	// 	}
	// }

	err = function_to_write(data_to_send)
	if err != nil {
		return err
	}
	return nil
}
