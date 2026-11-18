go tool pprof -top -diff_base="profiles/base.pprof" "profiles/result.pprof"

File: service.test.exe
Build ID: C:\Users\a-naumov\repos\go\url-shortener\internal\service\service.test.exe2026-11-13 16:07:56.5807757 +0300 MSK
Type: alloc_space
Time: 2026-11-13 15:24:35 MSK
Showing nodes accounting for 2505.04MB, 16.33% of 15336.43MB total
Dropped 111 nodes (cum <= 76.68MB)
      flat  flat%   sum%        cum   cum%
 6608.94MB 43.09% 43.09%  6608.94MB 43.09%  github.com/apnaumov/url-shortener.git/internal/repository.(*RuntimeStorage).GetUserURLs.func1
-3061.36MB 19.96% 23.13% -3482.37MB 22.71%  net/url.(*URL).joinPath
 2876.09MB 18.75% 41.89%  2876.09MB 18.75%  github.com/apnaumov/url-shortener.git/internal/service.(*URLShortenerService).buildShortURL (inline)
-2519.85MB 16.43% 25.45% -2519.85MB 16.43%  net/url.parse
-2308.61MB 15.05% 10.40% -2308.61MB 15.05%  maps.clone
 1364.13MB  8.89% 19.30%  1364.13MB  8.89%  github.com/apnaumov/url-shortener.git/internal/repository.(*Container[go.shape.struct { OriginalURL string "json:\"original_URL\""; CorrelationID string "json:\"correlation_id\""; UserID uint64 "json:\"user_id\""; IsDeleted bool "json:\"is_deleted\"" }]).Set
 -820.04MB  5.35% 13.95%  -820.04MB  5.35%  internal/bytealg.MakeNoZero
 -818.85MB  5.34%  8.61%  3526.81MB 23.00%  github.com/apnaumov/url-shortener.git/internal/repository.(*RuntimeStorage).GetUserURLs
  711.09MB  4.64% 13.25%   711.09MB  4.64%  github.com/apnaumov/url-shortener.git/internal/repository.(*Container[go.shape.string]).Set
  366.48MB  2.39% 15.64%   616.43MB  4.02%  github.com/apnaumov/url-shortener.git/internal/service.(*URLShortenerService).SetFullURLBatch
  313.76MB  2.05% 17.68%   984.41MB  6.42%  github.com/apnaumov/url-shortener.git/internal/service.BenchmarkSetFullURLBatch.func1
    -157MB  1.02% 16.66%  -421.01MB  2.75%  path.Join
    -138MB   0.9% 15.76%     -138MB   0.9%  path.(*lazybuf).string (inline)
    -126MB  0.82% 14.94%     -126MB  0.82%  path.(*lazybuf).append (inline)
  112.42MB  0.73% 15.67%  1886.65MB 12.30%  github.com/apnaumov/url-shortener.git/internal/repository.(*RuntimeStorage).SetURLBatch
   62.50MB  0.41% 16.08%    62.50MB  0.41%  internal/strconv.FormatUint
   17.50MB  0.11% 16.19%  -709.59MB  4.63%  github.com/apnaumov/url-shortener.git/internal/service.BenchmarkSetFullURL.func2.1
      15MB 0.098% 16.29%       15MB 0.098%  internal/strconv.FormatInt
    8.34MB 0.054% 16.34%   -37.47MB  0.24%  github.com/apnaumov/url-shortener.git/internal/repository.(*PendingMessageProcessor[go.shape.struct { UserID uint64; ShortURLs []string }]).Run.func1
   -1.50MB 0.0098% 16.33%  -708.59MB  4.62%  testing.(*B).RunParallel.func1
         0     0% 16.33% -2308.61MB 15.05%  github.com/apnaumov/url-shortener.git/internal/repository.(*Container[go.shape.struct { OriginalURL string "json:\"original_URL\""; CorrelationID string "json:\"correlation_id\""; UserID uint64 "json:\"user_id\""; IsDeleted bool "json:\"is_deleted\"" }]).GetAll
         0     0% 16.33%  6608.94MB 43.09%  github.com/apnaumov/url-shortener.git/internal/repository.(*Container[go.shape.struct { OriginalURL string "json:\"original_URL\""; CorrelationID string "json:\"correlation_id\""; UserID uint64 "json:\"user_id\""; IsDeleted bool "json:\"is_deleted\"" }]).Range
         0     0% 16.33%   -45.32MB   0.3%  github.com/apnaumov/url-shortener.git/internal/repository.(*PendingMessageProcessor[go.shape.struct { UserID uint64; ShortURLs []string }]).Run.func1.1
         0     0% 16.33%   300.98MB  1.96%  github.com/apnaumov/url-shortener.git/internal/repository.(*RuntimeStorage).SetURL
         0     0% 16.33%   -45.32MB   0.3%  github.com/apnaumov/url-shortener.git/internal/repository.(*RuntimeStorage).deleteURLsTickFunc
         0     0% 16.33%  2075.21MB 13.53%  github.com/apnaumov/url-shortener.git/internal/repository.(*RuntimeStorage).setURLImpl
         0     0% 16.33%  2311.47MB 15.07%  github.com/apnaumov/url-shortener.git/internal/service.(*URLShortenerService).GetUserURLs
         0     0% 16.33%  -782.15MB  5.10%  github.com/apnaumov/url-shortener.git/internal/service.(*URLShortenerService).SetFullURL
         0     0% 16.33%  2310.69MB 15.07%  github.com/apnaumov/url-shortener.git/internal/service.BenchmarkGetUserURLs.func1
         0     0% 16.33%       15MB 0.098%  internal/strconv.Itoa (inline)
         0     0% 16.33% -2308.61MB 15.05%  maps.Clone[go.shape.map[string]go.shape.struct { OriginalURL string "json:\"original_URL\""; CorrelationID string "json:\"correlation_id\""; UserID uint64 "json:\"user_id\""; IsDeleted bool "json:\"is_deleted\"" },go.shape.string,go.shape.struct { OriginalURL string "json:\"original_URL\""; CorrelationID string "json:\"correlation_id\""; UserID uint64 "json:\"user_id\""; IsDeleted bool "json:\"is_deleted\"" }] (inline)
         0     0% 16.33%  -820.04MB  5.35%  net/url.(*URL).String
         0     0% 16.33% -6822.25MB 44.48%  net/url.JoinPath
         0     0% 16.33% -2519.85MB 16.43%  net/url.Parse
         0     0% 16.33%     -264MB  1.72%  path.Clean
         0     0% 16.33%    62.50MB  0.41%  strconv.FormatUint (inline)
         0     0% 16.33%       15MB 0.098%  strconv.Itoa (inline)
         0     0% 16.33%  -820.04MB  5.35%  strings.(*Builder).Grow
         0     0% 16.33%  -820.04MB  5.35%  strings.(*Builder).grow
         0     0% 16.33%   -37.47MB  0.24%  sync.(*WaitGroup).Go.func1
         0     0% 16.33%  3297.79MB 21.50%  testing.(*B).run1.func1
         0     0% 16.33%  3298.76MB 21.51%  testing.(*B).runN
