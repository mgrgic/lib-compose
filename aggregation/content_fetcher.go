package aggregation

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

// FetchDefinition is a descriptor for fetching Content from an endpoint.
type FetchDefinition struct {
	URL            string
	Timeout        time.Duration
	RequestHeaders http.Header
	Required       bool
	//ServeResponseHeaders bool
	//IsPrimary            bool
	//FallbackURL string
}

func NewFetchDefinition(url string) *FetchDefinition {
	return &FetchDefinition{
		URL:            url,
		Timeout:        time.Second,
		RequestHeaders: nil,
		Required:       true,
	}
}

// Hash returns a unique hash for the fetch request.
// If two hashes of fetch resources are equal, they refer the same resource
// and can e.g. be taken as replacement for each other. E.g. in case of caching.
// TODO: Maybe we should exclude some headers from the hash?
func (def *FetchDefinition) Hash() string {
	hasher := md5.New()
	hasher.Write([]byte(def.URL))
	def.RequestHeaders.Write(hasher)
	return hex.EncodeToString(hasher.Sum(nil))
}

type FetchResult struct {
	Def     *FetchDefinition
	Err     error
	Content Content
	Hash    string // the hash of the FetchDefinition
}

// ContentFetcher is a type which can fetch a set of Content pages in parallel.
type ContentFetcher struct {
	activeJobs sync.WaitGroup
	r          struct {
		results []FetchResult
		mutex   sync.Mutex
	}
	contentLoader ContentLoader
}

// NewContentFetcher creates a ContentFetcher with an HtmlContentLoader as default.
// TODO: The FetchResults should always be returned in a predictable order,
// independent of the actual response times of the fetch jobs.
func NewContentFetcher() *ContentFetcher {
	f := &ContentFetcher{}
	f.r.results = make([]FetchResult, 0, 0)
	f.contentLoader = &HtmlContentLoader{}

	return f
}

// Wait blocks until all jobs are done,
// eighter sucessful or with an error result and returns the content and errors.
// Do we need to return the Results in a special order????
func (fetcher *ContentFetcher) WaitForResults() []FetchResult {
	fetcher.activeJobs.Wait()

	fetcher.r.mutex.Lock()
	defer fetcher.r.mutex.Unlock()

	return fetcher.r.results
}

// AddFetchJob addes one job to the fetcher and recursively adds the dependencies also.
func (fetcher *ContentFetcher) AddFetchJob(d *FetchDefinition) {
	fetcher.r.mutex.Lock()
	defer fetcher.r.mutex.Unlock()

	hash := d.Hash()
	if fetcher.isAlreadySheduled(hash) {
		return
	}
	fetchResult := FetchResult{Def: d, Hash: hash}
	fetcher.r.results = append(fetcher.r.results, fetchResult)

	fetcher.activeJobs.Add(1)
	go func() {
		defer fetcher.activeJobs.Done()

		fetchResult.Content, fetchResult.Err = fetcher.contentLoader.Load(d.URL, d.Timeout)

		for _, dependency := range fetchResult.Content.RequiredContent() {
			fetcher.AddFetchJob(dependency)
		}

	}()
}

// isAlreadySheduled checks, if there is already a job for a FetchDefinition, or it is already fetched.
// The method has to be called in a locked mutex block.
func (fetcher *ContentFetcher) isAlreadySheduled(fetchDefinitionHash string) bool {
	for _, fetchResult := range fetcher.r.results {
		if fetchDefinitionHash == fetchResult.Hash {
			return true
		}
	}
	return false
}