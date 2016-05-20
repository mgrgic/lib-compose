package composition

import (
	log "github.com/Sirupsen/logrus"
	"io"
	"net/http"
	"strings"
)

// A ContentFetcherFactory returns a configured fetch job for a request
// which can return the fetch results.
type ContentFetcherFactory func(r *http.Request) FetchResultSupplier

type CompositionHandler struct {
	contentFetcherFactory ContentFetcherFactory
	contentMergerFactory  func(metaJSON map[string]interface{}) ContentMerger
}

// NewCompositionHandler creates a new Handler with the supplied defualtData,
// which is used for each request.
func NewCompositionHandler(contentFetcherFactory ContentFetcherFactory) *CompositionHandler {
	return &CompositionHandler{
		contentFetcherFactory: contentFetcherFactory,
		contentMergerFactory: func(metaJSON map[string]interface{}) ContentMerger {
			return NewContentMerge(metaJSON)
		},
	}
}

func (agg *CompositionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	fetcher := agg.contentFetcherFactory(r)

	// fetch all contents
	results := fetcher.WaitForResults()

	mergeContext := agg.contentMergerFactory(fetcher.MetaJSON())

	for _, res := range results {
		if res.Err == nil && res.Content != nil {

			if res.Content.Reader() != nil {
				w.Header().Set("Content-Type", res.Content.HttpHeader().Get("Content-Type"))
				io.Copy(w, res.Content.Reader())
				res.Content.Reader().Close()
				return
			}

			mergeContext.AddContent(res)

		} else if res.Def.Required {
			log.WithField("fetchResult", res).Errorf("error loading content from: %v", res.Def.URL)
			http.Error(w, "Bad Gateway: "+res.Err.Error(), 502)
			return
		} else {
			log.WithField("fetchResult", res).Warnf("optional content not loaded: %v", res.Def.URL)
		}
	}

	if len(results) > 0 {
		// copy headers
		for k, values := range results[0].Content.HttpHeader() {
                        if k != "Content-Length" {
                                for _, v := range values {
                                        w.Header().Set(k, v)
                                }
                        }
		}
		if results[0].Content.HttpStatusCode() != 0 {
			w.WriteHeader(results[0].Content.HttpStatusCode())
		}
	}

	err := mergeContext.WriteHtml(w)
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), 500)
		return
	}
}

func MetadataForRequest(r *http.Request) map[string]interface{} {
	return map[string]interface{}{
		"host":     getHostFromRequest(r),
		"base_url": getBaseUrlFromRequest(r),
		"params":   r.URL.Query(),
	}
}

func getBaseUrlFromRequest(r *http.Request) string {
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	if xfph := r.Header.Get("X-Forwarded-Proto"); xfph != "" {
		protoParts := strings.SplitN(xfph, ",", 2)
		proto = protoParts[0]
	}

	return proto + "://" + getHostFromRequest(r)
}

func getHostFromRequest(r *http.Request) string {
	host := r.Host
	if xffh := r.Header.Get("X-Forwarded-For"); xffh != "" {
		hostParts := strings.SplitN(xffh, ",", 2)
		host = hostParts[0]
	}
	return host
}
