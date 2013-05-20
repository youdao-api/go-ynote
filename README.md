go-ynote
========

go-ynote 是[有道云笔记开放平台](http://note.youdao.com/open/) API 的 [Go](http://golang.org/) 语言封装。

用法
----

1) 建立一个<code>*YnoteClient</code>对象，设置开发者 token/secret.

```go
yc := ynote.NewOnlineYnoteClient(ynote.Credentials{
    Token:  "****",
    Secret: "****"})
```

2) 如果还没获得<code>AccToken</code>（存取令牌），如下方式得到：

```go
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

// save the accToken for further using.
```

3) 之后只要把保存下来的<code>AccToken</code>设置到<code>yc</code>的<code>AccToken</code>域就可以了（<code>RequestToken</code>如果成功会自动设置<code>AccToken</code>域）

```go
yc.AccToken = readAccToken()
```
		
4) 使用<code>yc</code>的操作方法，如 <code>UserInfo</code>/<code>ListNotebooks</code>等


LICENCE
-------
BSD license.
