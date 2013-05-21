/*
	ynote package encapsulates the open API of Youdao Note(ynote for short,
	note.youdao.com). The official site for open API is
	http://note.youdao.com/open/.

	Usage:

	1) Create a *YnoteClient instance with development token/secret.
		yc := ynote.NewOnlineYnoteClient(ynote.Credentials{
			Token:  "****",
			Secret: "****"})
	2) If we dont' have the access token, get it as follow
		tmpCred, err := yc.RequestTemporaryCredentials()
		if err != nil {
			return
		}
		fmt.Println("Temporary credentials got:", tmpCred)

		authUrl := yc.AuthorizationURL(tmpCred)
		// Let the end-user access this URL of authUrl using a browser,
		// authorize the request, and get a verifier.

		verifier := ... // Ask the end-user for the verifier

		accToken, err := yc.RequestToken(tmpCred, verifier)
		if err != nil {
			return
		}

		save the accToken for further using.
	3) If we read the access token from disk, Set it to the AccToken field of yc. (yc.RequestToken automatically set the field if success).
		yc.AccToken = readAccToken()

	4) Using yc's method to do operations.

*/
package ynote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/garyburd/go-oauth/oauth"
	"io/ioutil"
	//	"log"
	"errors"
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

/* Information of the ynote user. */
type UserInfo struct {
	// ID of the user
	ID string
	// The name of the user
	User string
	// The registration time
	RegisterTime time.Time
	// The last login time
	LastLoginTime time.Time
	// The modification time
	LastModifyTime time.Time
	// Total size in bytes
	TotalSize int64
	// Used size in bytes
	UsedSize int64
	// Path tho the default notbook
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
		return nil, errors.New("Response is not a JSON: " + string(js))
	}

	if res.StatusCode == 500 {
		return nil, parseFailInfo(js)
	}

	return &UserInfo{
		ID:              userInfo.ID,
		User:            userInfo.User,
		RegisterTime:    time.Unix(0, userInfo.RegisterTime*1000000000),
		LastLoginTime:   time.Unix(0, userInfo.LastLoginTime*1000000000),
		LastModifyTime:  time.Unix(0, userInfo.LastModifyTime*1000000000),
		TotalSize:       userInfo.TotalSize,
		UsedSize:        userInfo.UsedSize,
		DefaultNotebook: userInfo.DefaultNotebook,
	}, nil
}

/* The information of a notebook */
type NotebookInfo struct {
	// Name of the notebook
	Name string
	// Path to the notebook
	Path string
	// Number of notes in the notebook
	NotesNum int
	// Creation time
	CreateTime time.Time
	// Last modification time
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
		CreateTime: time.Unix(0, nbInfo.CreateTime*1000000000),
		ModifyTime: time.Unix(0, nbInfo.ModifyTime*1000000000),
		Path:       nbInfo.Path,
	}
}

func parseNotebookInfo(js []byte) (*NotebookInfo, error) {
	var nbInfo notebookInfo

	err := json.Unmarshal(js, &nbInfo)
	if err != nil {
		return nil, errors.New("Response is not a JSON: " + string(js))
	}

	return nbInfo.asNotebookInfo(), nil
}

/* The information for a failure calling. It is returned as an error. */
type FailInfo struct {
	Message string
	Err     string
}

/* Implementation of error.Error  */
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

/*
	CreateNotebook creates a new note book with specified name. A *NotebookInfo
	is returned if succeeds, non-nil error returned otherwise
*/
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
		return nil, errors.New("Response is not a JSON: " + string(js))
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

/*
	DeleteNotebook deletes a notebook. Returns nil if succeed, the error
	otherwise.
*/
func (yc *YnoteClient) DeleteNotebook(notebook string) error {
	reqUrl := yc.URLBase + "/yws/open/notebook/delete.json"

	params := make(url.Values)
	params.Set("notebook", notebook)

	res, err := yc.oauthClient.Post(http.DefaultClient, (*oauth.Credentials)(yc.AccToken), reqUrl, params)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	js, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode == 500 {
		return parseFailInfo(js)
	}

	return nil
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
		return "", errors.New("Response is not a JSON: " + string(js))
	}

	return path.Path, nil
}

/*
	ListNotes returns a list of path to all the notes in a notebook.
*/
func (yc *YnoteClient) ListNotes(notebookPath string) ([]string, error) {
	reqUrl := yc.URLBase + "/yws/open/notebook/list.json"

	params := make(url.Values)
	params.Set("notebook", notebookPath)

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

	var notes []string
	err = json.Unmarshal(js, &notes)
	if err != nil {
		return nil, errors.New("Response is not a JSON: " + string(js))
	}

	return notes, nil
}

/*
	NoteInfo is the datastructure storing information and content of a note.
*/
type NoteInfo struct {
	// Title of the note
	Title      string
	// Authro of the note
	Author     string
	// Source(URL) of the note
	Source     string
	// Size in bytes of the note
	Size       int64
	// Creation time
	CreateTime time.Time
	// Modification time
	ModifyTime time.Time
	// Content(HTML) of the note
	Content    string
}

/*
	NoteInfo returns the information and content of a note
*/
func (yc *YnoteClient) NoteInfo(path string) (*NoteInfo, error) {
	reqUrl := yc.URLBase + "/yws/open/note/get.json"

	params := make(url.Values)
	params.Set("path", path)

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

	var noteInfo struct {
		Title      string `json:"title"`
		Author     string `json:"author"`
		Source     string `json:"source"`
		Size       int64  `json:"size"`
		CreateTime int64  `json:"create_time"`
		ModifyTime int64  `json:"modify_time"`
		Content    string `json:"content"`
	}

	err = json.Unmarshal(js, &noteInfo)
	if err != nil {
		return nil, errors.New("Response is not a JSON: " + string(js))
	}

	return &NoteInfo{
		Title:      noteInfo.Title,
		Author:     noteInfo.Author,
		Source:     noteInfo.Source,
		Size:       noteInfo.Size,
		CreateTime: time.Unix(0, noteInfo.CreateTime*1000000000),
		ModifyTime: time.Unix(0, noteInfo.ModifyTime*1000000000),
		Content:    noteInfo.Content,
	}, nil
}

/*
	UpdateNote modifies the title/author/source/content of a note
*/
func (yc *YnoteClient) UpdateNote(path, title, author, source, content string) (error) {
	reqUrl := yc.URLBase + "/yws/open/note/update.json"

	params := make(url.Values)
	params.Set("path", path)
	params.Set("title", title)
	params.Set("author", author)
	params.Set("source", source)
	params.Set("content", content)

	res, err := multipartPost(&yc.oauthClient, http.DefaultClient, (*oauth.Credentials)(yc.AccToken), reqUrl, params)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	js, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode == 500 {
		return parseFailInfo(js)
	}

	return nil
}

/*
	DeleteNote deletes a note
*/
func (yc *YnoteClient) DeleteNote(path string) error {
	reqUrl := yc.URLBase + "/yws/open/note/delete.json"

	params := make(url.Values)
	params.Set("path", path)

	res, err := yc.oauthClient.Post(http.DefaultClient, (*oauth.Credentials)(yc.AccToken), reqUrl, params)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	js, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode == 500 {
		return parseFailInfo(js)
	}

	return nil
}

/*
	MoveNote moves a note into another notebook
*/
func (yc *YnoteClient) MoveNote(notePath, notebookPath string) error {
	reqUrl := yc.URLBase + "/yws/open/note/move.json"

	params := make(url.Values)
	params.Set("path", notePath)
	params.Set("notebook", notebookPath)

	res, err := yc.oauthClient.Post(http.DefaultClient, (*oauth.Credentials)(yc.AccToken), reqUrl, params)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	js, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode == 500 {
		return parseFailInfo(js)
	}

	return nil
}
