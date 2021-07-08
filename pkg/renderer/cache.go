package renderer

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"arhat.dev/pkg/hashhelper"
	lru "github.com/die-net/lrucache"

	"arhat.dev/dukkha/pkg/field"
	"arhat.dev/dukkha/pkg/utils"
)

type CacheConfig struct {
	field.BaseField

	EnableCache    bool          `yaml:"enable_cache"`
	CacheSizeLimit utils.Size    `yaml:"cache_size_limit"`
	CacheMaxAge    time.Duration `yaml:"cache_max_age"`
}

type CacheRefreshFunc func(key string) ([]byte, error)

func NewCache(
	limit int64,
	expiry time.Duration,
	refresh CacheRefreshFunc,
) *Cache {
	return &Cache{
		refresh: refresh,
		cache:   lru.New(limit, int64(expiry.Seconds())),
	}
}

type Cache struct {
	refresh CacheRefreshFunc
	cache   *lru.LruCache
}

func (c *Cache) Get(key string) ([]byte, error) {
	data, ok := c.cache.Get(key)
	if ok {
		return data, nil
	}

	data, err := c.refresh(key)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, data)
	return data, nil
}

func CreateFetchFunc(
	dukkhaCacheDir, rendererName string,
	maxCacheAge time.Duration,
	doRemoteFetch CacheRefreshFunc,
) CacheRefreshFunc {
	return func(key string) ([]byte, error) {
		localCacheFilePrefix := hex.EncodeToString(hashhelper.MD5Sum([]byte(key)))
		localCacheDir := filepath.Join(
			dukkhaCacheDir, fmt.Sprintf("renderer-%s", rendererName),
		)

		// find from local cache
		// ${DUKKHA_CACHE_DIR}/renderer-<rendererName>/<md5sum(key)>-<unix-timestamp>

		entries, err := os.ReadDir(localCacheDir)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf(
					"failed to check local cache dir for %q: %w",
					rendererName, err,
				)
			}

			// no cache exists

			if os.IsNotExist(err) {
				err = os.MkdirAll(localCacheDir, 0750)
				if err != nil {
					return nil, fmt.Errorf(
						"failed to ensure local cache dir for renderer %q: %w",
						rendererName, err,
					)
				}
			}
			// fetch from remote
		} else {
			start := sort.Search(len(entries), func(i int) bool {
				return strings.HasPrefix(entries[i].Name(), localCacheFilePrefix)
			})

			if start >= 0 {
				latestAt := start + 1
				for latestAt < len(entries) && strings.HasPrefix(entries[latestAt].Name(), localCacheFilePrefix) {
					latestAt++
				}

				for _, info := range entries[start:latestAt] {
					_ = os.Remove(filepath.Join(localCacheDir, info.Name()))
				}

				targetFile := entries[latestAt].Name()
				targetPath := filepath.Join(localCacheDir, targetFile)

				parts := strings.SplitN(targetFile, "-", 2)
				if len(parts) != 2 || parts[0] != localCacheFilePrefix {
					// invalid cache file
					return nil, fmt.Errorf(
						"invalid cache file, please remove %q: %w",
						targetPath, err,
					)
				}

				timestamp, err2 := strconv.ParseInt(
					// trim padding
					strings.TrimLeft(parts[1], "0"),
					10, 64,
				)
				if err2 != nil {
					return nil, fmt.Errorf(
						"invalid timestamp, please remove local cache file %q: %w",
						targetPath, err2,
					)
				}

				if time.Since(time.Unix(timestamp, 0)) < maxCacheAge {
					return os.ReadFile(targetPath)
				}

				// cache expired
				_ = os.Remove(targetPath)
			}
		}

		// pad timestamp to get sorted by os.ReadDir
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		// int64 can have at most 20 digits
		timestamp = strings.Repeat("0", 20-len(timestamp))

		localCacheFile := filepath.Join(
			localCacheDir,
			fmt.Sprintf("%s-%s", localCacheFilePrefix, timestamp),
		)

		data, err := doRemoteFetch(key)
		if err != nil {
			return nil, err
		}

		err = os.WriteFile(localCacheFile, data, 0640)
		if err != nil {
			_ = os.Remove(localCacheFile)
			return nil, err
		}

		return data, nil
	}
}