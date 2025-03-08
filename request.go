package gohttp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type (
	BeforeRequestHook func(*Request) error
	AfterResponseHook func(*Response) error
	ErrorHook         func(*Request, error)
)

// Request is a request type
type Request struct {
	transport              *http.Transport
	client                 *http.Client
	cookie                 http.CookieJar
	timeout                time.Duration
	formVals               *bytes.Buffer
	multipartBuffer        bytes.Buffer
	queryVals              string
	headers                map[string]string
	writer                 *multipart.Writer
	contentType            string
	basicUser, basicPasswd string
	beforeRequestHooks     []BeforeRequestHook
	afterResponseHooks     []AfterResponseHook
	errorHooks             []ErrorHook
	ctx                    context.Context
}

// MultipartParam is a multipart param type
type MultipartParam struct {
	FieldName string
	FileName  string
	FileBody  io.Reader
}

// NewRequest returns a new request
func NewRequest(opts ...Option) *Request {
	r := &Request{}
	for _, o := range opts {
		o.apply(r)
	}
	return r
}

// createClient create request client
func (req *Request) createClient() *http.Client {
	tr := req.transport
	if tr == nil {
		tr = http.DefaultTransport.(*http.Transport)
	}

	if req.client == nil {
		req.client = &http.Client{
			Transport: tr,
			Timeout:   req.timeout,
			Jar:       req.cookie,
		}
	}

	return req.client
}

// JSON set json data with request
func (req *Request) JSON(jsonBody map[string]interface{}) *Request {

	data, err := json.Marshal(jsonBody)
	if err != nil {
		panic(err)
	}

	req.formVals = bytes.NewBuffer(data)
	req.contentType = "application/json"
	return req
}

// FormData set Post request form parameters
func (req *Request) FormData(formValues map[string]string) *Request {
	vals := url.Values{}
	for key, val := range formValues {
		vals.Add(key, val)
	}

	req.formVals = bytes.NewBuffer([]byte(vals.Encode()))
	req.contentType = "application/x-www-form-urlencoded"

	return req
}

// Body set Post request as body
func (req *Request) Body(formValues []byte) *Request {

	req.formVals = bytes.NewBuffer(formValues)
	req.contentType = "application/octet-stream"

	return req
}

// Text is send text data with post request
func (req *Request) Text(formValues string) *Request {

	req.formVals = bytes.NewBuffer([]byte(formValues))
	req.contentType = "text/plain"

	return req
}

// Query set request query param
func (req *Request) Query(formValues map[string]string) *Request {
	vals := url.Values{}
	for key, val := range formValues {
		vals.Add(key, val)
	}

	req.queryVals = vals.Encode()
	req.contentType = "application/x-www-form-urlencoded"

	return req
}

// Headers set header information
func (req *Request) Headers(headerVals map[string]string) *Request {
	req.headers = headerVals
	return req
}

// BasicAuth make basic authentication
func (req *Request) BasicAuth(username, password string) *Request {
	req.basicUser = username
	req.basicPasswd = password

	return req
}

// Get is a get http request
func (req *Request) Get(url string) (*Response, error) {
	return req.makeRequest(http.MethodGet, url, req.formVals)
}

// Post is a post http request
func (req *Request) Post(url string) (*Response, error) {
	return req.makeRequest(http.MethodPost, url, req.formVals)
}

// Put is a put http request
func (req *Request) Put(url string) (*Response, error) {
	return req.makeRequest(http.MethodPut, url, req.formVals)
}

// Patch is a patch http request
func (req *Request) Patch(url string) (*Response, error) {
	return req.makeRequest(http.MethodPatch, url, req.formVals)
}

// Delete is a delete http request
func (req *Request) Delete(url string) (*Response, error) {
	return req.makeRequest(http.MethodDelete, url, req.formVals)
}

// Head is a head http request
func (req *Request) Head(url string) (*Response, error) {
	return req.makeRequest(http.MethodHead, url, req.formVals)
}

// Options is a options http request
func (req *Request) Options(url string) (*Response, error) {
	return req.makeRequest(http.MethodOptions, url, req.formVals)
}

// MultipartFormData add form data in multipart request
func (req *Request) MultipartFormData(formData map[string]string) *Request {
	if req.writer == nil {
		req.writer = multipart.NewWriter(&req.multipartBuffer)
	}

	for key, val := range formData {
		req.writer.WriteField(key, val)
	}
	return req
}

// Upload upload a single file
func (req *Request) Upload(name, file string) *Request {
	if req.writer == nil {
		req.writer = multipart.NewWriter(&req.multipartBuffer)
	}

	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// Add file
	fw, err := req.writer.CreateFormFile(name, file)
	if err != nil {
		panic(err)
	}
	if _, err = io.Copy(fw, f); err != nil {
		panic(err)
	}

	req.contentType = req.writer.FormDataContentType()
	req.formVals = &req.multipartBuffer
	return req
}

// UploadFromReader upload a single file
func (req *Request) UploadFromReader(param MultipartParam) *Request {
	if req.writer == nil {
		req.writer = multipart.NewWriter(&req.multipartBuffer)
	}

	// Add file
	fw, err := req.writer.CreateFormFile(param.FieldName, param.FileName)
	if err != nil {
		panic(err)
	}
	if _, err = io.Copy(fw, param.FileBody); err != nil {
		panic(err)
	}

	req.contentType = req.writer.FormDataContentType()
	req.formVals = &req.multipartBuffer
	return req
}

// Uploads upload multiple files
func (req *Request) Uploads(files map[string]string) *Request {

	for name, file := range files {
		_ = req.Upload(name, file)
	}

	return req
}

// UploadsFromReader upload multiple files
func (req *Request) UploadsFromReader(params []MultipartParam) *Request {

	for _, param := range params {
		_ = req.UploadFromReader(param)
	}

	return req
}

func (req *Request) OnBeforeRequest(hook BeforeRequestHook) *Request {
	req.beforeRequestHooks = append(req.beforeRequestHooks, hook)
	return req
}

func (req *Request) OnAfterResponse(hook AfterResponseHook) *Request {

	req.afterResponseHooks = append(req.afterResponseHooks, hook)
	return req
}
func (req *Request) OnError(errorHook ErrorHook) *Request {
	req.errorHooks = append(req.errorHooks, errorHook)
	return req
}

func (req *Request) ExecuteBeforeRequestHooks() {
	for _, beforeReqHook := range req.beforeRequestHooks {
		beforeReqHook(req)
	}
}

func (req *Request) ExecuteAfterResponseHooks(response Response) {
	for _, afterResponseHook := range req.afterResponseHooks {
		afterResponseHook(&response)
	}
}

func (req *Request) ExecuteOnErrorHooks(err error) {
	for _, errorHooks := range req.errorHooks {
		errorHooks(req, err)
	}
}

// Context method returns the Context if it is already set in the [Request]
// otherwise, it creates a new one using [context.Background].
func (r *Request) Context() context.Context {
	if r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

// SetContext method sets the [context.Context] for current [Request]. It allows
// to interrupt the request execution if `ctx.Done()` channel is closed.
// See https://blog.golang.org/context article and the package [context]
// documentation.
func (r *Request) SetContext(ctx context.Context) *Request {
	r.ctx = ctx
	return r
}

// makeRequest makes a http request
func (req *Request) makeRequest(verb, url string, payloads *bytes.Buffer) (*Response, error) {
	req.ExecuteBeforeRequestHooks()

	response := Response{}
	verb = strings.ToUpper(verb)
	var request *http.Request
	var err error
	client := req.createClient()

	if req.writer != nil {
		req.writer.Close()
	}
	if req.queryVals != "" {
		url += "?" + req.queryVals
	}

	if payloads == nil {
		payloads = bytes.NewBuffer([]byte(``))
	}

	if verb == "GET" {
		request, err = http.NewRequest(verb, url, nil)
	} else {
		request, err = http.NewRequest(verb, url, payloads)
	}

	if err != nil {
		req.ExecuteOnErrorHooks(err)
		return nil, err
	}

	request.Header.Set("Content-Type", req.contentType)

	if req.basicUser != "" && req.basicPasswd != "" {
		request.SetBasicAuth(req.basicUser, req.basicPasswd)
	}

	// set headers from Headers method
	for key, val := range req.headers {
		request.Header.Set(key, val)
	}

	if val, ok := req.headers["Host"]; ok {
		request.Host = val
	}
	//request.Close = true
	resp, err := client.Do(request)

	if err != nil {
		req.ExecuteOnErrorHooks(err)
		return nil, err
	}
	response.resp = resp
	req.ExecuteAfterResponseHooks(response)

	return &response, nil
}
