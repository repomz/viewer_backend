package httpserver

import (
	"sync"
	"time"
)

const (
	operationPlanCacheTTL = 30 * time.Second
	statisticsCacheTTL    = 60 * time.Second
)

type cachedOperationPlanResponse struct {
	value     operationPlanResponse
	expiresAt time.Time
}

var operationPlanResponseCache = struct {
	sync.Mutex
	entries map[string]cachedOperationPlanResponse
}{entries: make(map[string]cachedOperationPlanResponse)}

type cachedStatisticsResponse struct {
	value     operationStatisticsResponse
	expiresAt time.Time
}

var operationStatisticsResponseCache = struct {
	sync.Mutex
	entries map[string]cachedStatisticsResponse
}{entries: make(map[string]cachedStatisticsResponse)}

func invalidateOperationPlanResponseCache() {
	operationPlanResponseCache.Lock()
	operationPlanResponseCache.entries = make(map[string]cachedOperationPlanResponse)
	operationPlanResponseCache.Unlock()
}

func invalidateOperationStatisticsResponseCache() {
	operationStatisticsResponseCache.Lock()
	operationStatisticsResponseCache.entries = make(map[string]cachedStatisticsResponse)
	operationStatisticsResponseCache.Unlock()
}

func invalidateStudyAnalysisResponseCaches() {
	invalidateOperationPlanResponseCache()
	invalidateOperationStatisticsResponseCache()
}
