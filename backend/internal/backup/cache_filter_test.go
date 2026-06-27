package backup

import "testing"

func TestIsChromiumCacheRelPath_SkipsCache(t *testing.T) {
	skip := []string{
		"profile1/Default/Cache/data_0",
		"profile1/Default/Code Cache/js/index",
		"profile1/Default/GPUCache/data_1",
		"profile1/Default/Service Worker/CacheStorage/abc",
		"profile1/Default/Service Worker/ScriptCache/def",
		"profile1/Default/Network/Cache/index",
		"profile1/Crashpad/reports/x.dmp",
		"profile1/Default/DawnGraphiteCache/00",
		"profile1/component_crx_cache/aaa",
		"Default\\Cache\\data_2", // 系统分隔符
	}
	for _, rel := range skip {
		if !IsChromiumCacheRelPath(rel) {
			t.Errorf("应判定为缓存但未命中: %s", rel)
		}
	}
}

func TestIsChromiumCacheRelPath_KeepsPersistentData(t *testing.T) {
	keep := []string{
		"profile1/Default/Cookies",
		"profile1/Default/Login Data",
		"profile1/Default/Preferences",
		"profile1/Default/Local Storage/leveldb/000003.log",
		"profile1/Default/IndexedDB/https_example/000001.ldb",
		"profile1/Default/Service Worker/Database/000005.log",
		"profile1/Default/Bookmarks",
		"profile1/Default/History",
		"app.db",
		"",
	}
	for _, rel := range keep {
		if IsChromiumCacheRelPath(rel) {
			t.Errorf("应保留但被判定为缓存: %s", rel)
		}
	}
}
