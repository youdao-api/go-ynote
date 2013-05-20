package ynote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/garyburd/go-oauth/oauth"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

/* The URL base for online ynote service */
var OnlineUrlBase = "http://note.youdao.com"

/* A database for storing credential information */
type Credentials oauth.Credentials

/* The type for a ynote client */
type YnoteClient struct {
	// The URL base
	URLBase     string
	oauthClient oauth.Client
	// The access token
	AccToken *Credentials
}

/*
	NewOnlineYnoteClient creates a *YnoteClient for online service.
*/
func NewOnlineYnoteClient(credentials Credentials) *YnoteClient {
	return NewYnoteClient(credentials, OnlineUrlBase)
}

/*
	NewOnlineYnoteClient creates a *YnoteClient for a service with speicified
	URLBase.
*/
func NewYnoteClient(credentials Credentials, urlBase string) *YnoteClient {
	return &YnoteClient{
		URLBase: urlBase,
		oauthClient: oauth.Client{
			Credentials:                   oauth.Credentials(credentials),
			TemporaryCredentialRequestURI: urlBase + "/oauth/request_token",
			ResourceOwnerAuthorizationURI: urlBase + "/oauth/authorize",
			TokenRequestURI:               urlBase + "/oauth/access_token",
		},
	}
}

/*
	RequestTemporaryCredentials requests a temporary token
*/
func (yc *YnoteClient) RequestTemporaryCredentials() (*Credentials, error) {
	tmpCred, err := yc.oauthClient.RequestTemporaryCredentials(http.DefaultClient, "", nil)
	if err != nil {
		return nil, err
	}
	return (*Credentials)(tmpCred), nil

}

/*
	RequestTemporaryCredentials returns the autorization URL
*/
func (yc *YnoteClient) AuthorizationURL(tmpCred *Credentials) string {
	return yc.oauthClient.AuthorizationURL((*oauth.Credentials)(tmpCred), nil)
}

/*
	RequestTemporaryCredentials returns the access token given the verifier
*/
func (yc *YnoteClient) RequestToken(tmpCred *Credentials, verifier string) (accToken *Credentials, err error) {
	token, _, err := yc.oauthClient.RequestToken(http.DefaultClient, (*oauth.Credentials)(tmpCred), verifier)
	if err != nil {
		return nil, err
	}
	yc.AccToken = (*Credentials)(token)
	return yc.AccToken, err
}

/* UserInfo */
type UserInfo struct {
	ID              string
	User            string
	RegisterTime    time.Time
	LastLoginTime   time.Time
	LastModifyTime  time.Time
	TotalSize       int64
	UsedSize        int64
	DefaultNotebook string
}

/*
	UserInfo fetches the information of the ynote user
*/
func (yc *YnoteClient) UserInfo() (ui *UserInfo, err error) {
	reqUrl := yc.URLBase + "/yws/open/user/get.json"
	res, err := yc.oauthClient.Get(http.DefaultClient, (*oauth.Credentials)(yc.AccToken), reqUrl, nil)
	if err != nil {
		return nil, err
	}
	js, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	//	fmt.Println(string(result))

	var userInfo struct {
		ID              string `json:"id"`
		User            string `json:"user"`
		RegisterTime    int64  `json:"register_time"`
		LastLoginTime   int64  `json:"last_login_time"`
		LastModifyTime  int64  `json:"last_modify_time"`
		TotalSize       int64  `json:"total_size"`
		UsedSize        int64  `json:"used_size"`
		DefaultNotebook string `json:"default_notebook"`
	}
	err = json.Unmarshal(js, &userInfo)
	if err != nil {
		return nil, err
	}

	if res.StatusCode == 500 {
		return nil, parseFailInfo(js)
	}

	return &UserInfo{
		ID:              userInfo.ID,
		User:            userInfo.User,
		RegisterTime:    time.Unix(0, userInfo.RegisterTime*1000000),
		LastLoginTime:   time.Unix(0, userInfo.LastLoginTime*1000000),
		LastModifyTime:  time.Unix(0, userInfo.LastModifyTime*1000000),
		TotalSize:       userInfo.TotalSize,
		UsedSize:        userInfo.UsedSize,
		DefaultNotebook: userInfo.DefaultNotebook,
	}, nil
}

type NotebookInfo struct {
	Name       string
	Path       string
	NotesNum   int
	CreateTime time.Time
	ModifyTime time.Time
}

func (ni *NotebookInfo) String() string {
	return fmt.Sprintf("%+v", *ni)
}

type notebookInfo struct {
	NotesNum   int    `json:"notes_num"`
	Name       string `json:"name"`
	CreateTime int64  `json:"create_time"`
	ModifyTime int64  `json:"modify_time"`
	Path       string `json:"path"`
}

func (nbInfo *notebookInfo) asNotebookInfo() *NotebookInfo {
	return &NotebookInfo{
		NotesNum:   nbInfo.NotesNum,
		Name:       nbInfo.Name,
		CreateTime: time.Unix(0, nbInfo.CreateTime*1000000),
		ModifyTime: time.Unix(0, nbInfo.ModifyTime*1000000),
		Path:       nbInfo.Path,
	}
}

func parseNotebookInfo(js []byte) (*NotebookInfo, error) {
	var nbInfo notebookInfo

	err := json.Unmarshal(js, &nbInfo)
	if err != nil {
		return nil, err
	}

	return nbInfo.asNotebookInfo(), nil
}

type FailInfo struct {
	Message string
	Err     string
}

func (info *FailInfo) Error() string {
	return fmt.Sprintf("%s: %s", info.Err, info.Message)
}

func parseFailInfo(js []byte) *FailInfo {
	var failInfo struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}

	err := json.Unmarshal(js, &failInfo)
	if err != nil {
		return &FailInfo{
			Message: "Parse FailInfo failed: " + string(js),
			Err:     "Unknown",
		}
	}

	return &FailInfo{
		Message: failInfo.Message,
		Err:     failInfo.Error,
	}
}

/* CreateNotebook creates a new note book with specified name. An NotebookInfo
is returned if succeeds, non-nil error returned otherwise */
func (yc *YnoteClient) CreateNotebook(name string) (*NotebookInfo, error) {
	reqUrl := yc.URLBase + "/yws/open/notebook/create.json"
	params := make(url.Values)
	params.Set("name", name)
	res, err := yc.oauthClient.Post(http.DefaultClient, (*oauth.Credentials)(yc.AccToken), reqUrl, params)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	js, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == 500 {
		return nil, parseFailInfo(js)
	}
	return parseNotebookInfo(js)
}

/*
	ListNotebooks returns all notebooks.
*/
func (yc *YnoteClient) ListNotebooks() ([]*NotebookInfo, error) {
	reqUrl := yc.URLBase + "/yws/open/notebook/all.json"
	res, err := yc.oauthClient.Post(http.DefaultClient, (*oauth.Credentials)(yc.AccToken), reqUrl, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	js, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == 500 {
		return nil, parseFailInfo(js)
	}

	var nbInfos []notebookInfo
	err = json.Unmarshal(js, &nbInfos)
	if err != nil {
		log.Println("Response is not a JSON:", string(js))
		return nil, err
	}
	nbs := make([]*NotebookInfo, 0, len(nbInfos))
	for _, nb := range nbInfos {
		nbs = append(nbs, nb.asNotebookInfo())
	}

	return nbs, nil
}

/*
	FindNotebook returns the NotebookInfo of the speicified name, or nil if not found.
*/
func (yc *YnoteClient) FindNotebook(name string) (*NotebookInfo, error) {
	nbs, err := yc.ListNotebooks()
	if err != nil {
		return nil, err
	}

	for _, nb := range nbs {
		if nb.Name == name {
			return nb, nil
		}
	}
	return nil, nil
}

// Post issues a POST with the specified form.
func multipartPost(c *oauth.Client, client *http.Client, credentials *oauth.Credentials, urlStr string, form url.Values) (*http.Response, error) {
	var bf = &bytes.Buffer{}
	mw := multipart.NewWriter(bf)
	contentType := mw.FormDataContentType()
	for k := range form {
		mw.WriteField(k, form.Get(k))
	}
	mw.Close()

	req, err := http.NewRequest("POST", urlStr, bf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)

	req.Header.Set("Authorization", c.AuthorizationHeader(credentials, "POST", req.URL, nil))
	return client.Do(req)
}

/*
	CreateNote creates a new note in a speicifed notebookPath. The path to the
	new note is returned if succeed.
*/
func (yc *YnoteClient) CreateNote(notebookPath, title, author, source, content string) (string, error) {
	reqUrl := yc.URLBase + "/yws/open/note/create.json"

	params := make(url.Values)
	params.Set("notebook", notebookPath)
	params.Set("title", title)
	params.Set("author", author)
	params.Set("source", source)
	params.Set("content", content)

	res, err := multipartPost(&yc.oauthClient, http.DefaultClient, (*oauth.Credentials)(yc.AccToken), reqUrl, params)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	js, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode == 500 {
		return "", parseFailInfo(js)
	}

	var path struct {
		Path string `json:"path"`
	}
	err = json.Unmarshal(js, &path)
	if err != nil {
		log.Println("Response is not a JSON:", string(js))
		return "", err
	}

	return path.Path, nil
}
