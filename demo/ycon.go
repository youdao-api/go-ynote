package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/daviddengcn/go-villa"
	ynote "github.com/youdao-api/go-ynote"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const ac_FILENAME = villa.Path("at.json")

func readAccToken() *ynote.Credentials {
	js, err := ac_FILENAME.ReadFile()
	if err != nil {
		return nil
	}

	var cred ynote.Credentials
	err = json.Unmarshal(js, &cred)
	if err != nil {
		return nil
	}
	return &cred
}

func saveAccToken(ac *ynote.Credentials) {
	js, err := json.Marshal(ac)
	if err != nil {
		log.Fatal("Marshal accToken failed:", err)
		return
	}
	err = ac_FILENAME.WriteFile(js, 0666)
	if err != nil {
		log.Fatal("Write accToken failed:", err)
	}
}

func requestForAccess(yc *ynote.YnoteClient) {
	fmt.Println("Access token (" + ac_FILENAME +
		") not found, try authorize...")
	fmt.Println("Requesting temporary credentials ...")
	tmpCred, err := yc.RequestTemporaryCredentials()
	if err != nil {
		log.Fatal("RequestTemporaryCredentials failed: ", err)
	}
	fmt.Println("Temporary credentials got:", tmpCred)

	authUrl := yc.AuthorizationURL(tmpCred)
	fmt.Println(authUrl)
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", authUrl).Start()
	case "windows":
		exec.Command("cmd", "/d", "/c", "start", authUrl).Start()
	case "linux":
		exec.Command("xdg-open", authUrl).Start()
	}

	fmt.Print("Please input the verifier: ")
	verifier, err := bufio.NewReader(os.Stdin).ReadString('\n')

	if err != nil {
		log.Fatal("Read verifier from console failed: ", err)
	}

	verifier = strings.TrimSpace(verifier)
	fmt.Println("verifier:", verifier)

	accToken, err := yc.RequestToken(tmpCred, verifier)
	if err != nil {
		log.Fatal("RequestToken failed: ", err)
		return
	}

	fmt.Println(accToken)
	saveAccToken(accToken)
}

func main() {
	yc := ynote.NewOnlineYnoteClient(ynote.Credentials{
		Token:  "e13d9c47ee9f332c2cb53828e81c5e8f",
		Secret: "3e37b6c79413014d482e4e00b86a041f"})

	yc.AccToken = readAccToken()

	if yc.AccToken == nil {
		requestForAccess(yc)
	}

	ui, err := yc.UserInfo()
	if err != nil {
		if fi, ok := err.(*ynote.FailInfo); ok && fi.Err == "1007" {
			// Maybe token changed
			requestForAccess(yc)
			// Try fetch userinfo again
			ui, err = yc.UserInfo()
			if err != nil {
				log.Fatal("UserInfo failed:", err)
			}
		} else {
			log.Fatal("UserInfo failed:", err)
		}
	}
	fmt.Printf("Hi, %s(last login at %v)\n", ui.User, ui.LastLoginTime)

	const (
		pos_ALL = iota
		pos_NOTEBOOK
		pos_NOTE
	)

	status := pos_ALL
	var notebook *ynote.NotebookInfo
	var notePath string

mainloop:
	for {
		switch status {
		case pos_ALL:
			nbs, err := yc.ListNotebooks()
			if err != nil {
				fmt.Println("ListNotebooks failed:", err)
				break mainloop
			}

			// sort the notebooks
			villa.SortF(len(nbs), func(i, j int) bool {
				nbi, nbj := nbs[i], nbs[j]
				/*
					if nbi.Group != nbj.Group {
						if nbj.Group == "" || nbi.Group < nbj.Group {
							return true
						}

						return false
					}
				*/

				if nbi.Group != nbj.Group {
					if nbj.Group == "" {
						return true
					}
					if nbi.Group == "" {
						return false
					}

					return nbi.Group < nbj.Group
				}

				return nbi.Name < nbj.Name
			}, func(i, j int) {
				nbs[i], nbs[j] = nbs[j], nbs[i]
			})

			fmt.Println("All notebooks:")
			for i, nb := range nbs {
				if nb.Group != "" && (i == 0 || nb.Group != nbs[i-1].Group) {
					fmt.Printf("    + %s\n", nb.Group)
				}
				if nb.Group == "" {
					fmt.Printf("%2d: %s(%d)\n", i+1, nb.Name, nb.NotesNum)
				} else {
					fmt.Printf("%2d:     %s(%d)\n", i+1, nb.Name, nb.NotesNum)
				}
			}

			if len(nbs) > 0 {
				fmt.Printf("%d-%d: View notebook, ", 1, len(nbs))
			}
			fmt.Println("q: quit")
			cmd, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				fmt.Println("Read console failed:", err)
				break mainloop
			}

			cmd = strings.TrimSpace(cmd)
			switch cmd {
			case "q":
				break mainloop
			default:
				idx, err := strconv.Atoi(cmd)
				if err == nil && idx >= 1 && idx <= len(nbs) {
					status = pos_NOTEBOOK
					notebook = nbs[idx-1]
				}
			}
		case pos_NOTEBOOK:
			fmt.Println("Notebook:", notebook.Name)
			notes, err := yc.ListNotes(notebook.Path)
			if err != nil {
				fmt.Println("ListNotes failed:", err)
				break mainloop
			}

			for i, note := range notes {
				if i >= 50 {
					// only show top 20
					break
				}
				ni, err := yc.NoteInfo(note)
				if err != nil {
					fmt.Printf("%2d: (path)%s\n", i+1, note)
				} else {
					fmt.Printf("%2d: %s\n", i+1, ni.Title)
				}
			}

			if len(notes) > 0 {
				fmt.Printf("%d-%d: View notebook, ", 1, len(notes))
			}
			fmt.Println("a: all notebooks, q: quit, " +
				"delete: delete the nootbook, put <filename>: add a note " +
				"with a file as its attachment.")
			cmd, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				fmt.Println("Read console failed:", err)
				break mainloop
			}

			cmd = strings.TrimSpace(cmd)
			switch cmd {
			case "a":
				status = pos_ALL
			case "q":
				break mainloop
			case "delete":
				err := yc.DeleteNotebook(notebook.Path)
				if err != nil {
					fmt.Println("DeleteNotebook failed", err)
					break
				}
				fmt.Println("DeleteNotebook succeed")
				status = pos_ALL
			default:
				if strings.HasPrefix(cmd, "put ") {
					fn := strings.TrimSpace(cmd[len("put "):])
					if len(fn) > 0 {
						ai, err := yc.UploadAttachment(fn)
						if err != nil {
							fmt.Println("UploadAttachment failed:", err)
							break
						}

						var content string
						if ai.Src == "" {
							// an image
							content = fmt.Sprintf(`<img src="%s">`,
								ai.URL)
						} else {
							// common attachment
							content = fmt.Sprintf(`<img path="%s" src="%s">`,
								ai.URL, ai.Src)
						}
						path, err := yc.CreateNote(notebook.Path, fn, "ycon",
							"", content)
						if err != nil {
							fmt.Println("CreateNote failed:", err)
							break
						}
						fmt.Println("CreateNote:", path)
						break
					}
				}
				idx, err := strconv.Atoi(cmd)
				if err == nil && idx >= 1 && idx <= len(notes) {
					status = pos_NOTE
					notePath = notes[idx-1]
				}
			}

		case pos_NOTE:
			fmt.Println("Note:", notePath)

			ni, err := yc.NoteInfo(notePath)
			if err != nil {
				fmt.Println("NoteInfo failed:", err)
				status = pos_NOTEBOOK
				continue
			}

			fmt.Printf("Title     : %s\n", ni.Title)
			fmt.Printf("Author    : %s\n", ni.Author)
			fmt.Printf("Source    : %s\n", ni.Source)
			fmt.Printf("Size      : %d bytes\n", ni.Size)
			fmt.Printf("CreateTime: %s\n", ni.CreateTime.Format("2006-01-02 15:04:05"))
			fmt.Printf("ModifyTime: %s\n", ni.ModifyTime.Format("2006-01-02 15:04:05"))
			fmt.Printf("Content   : %d bytes\n", len(ni.Content))

			fmt.Println("a: all notebooks, n: notebook, q: quit, delete: " +
				"delete current note, title/author/source <content>: change " +
				"title/author/source, content: show content, adl <link>: " +
				"authorize download link")
			cmd, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				fmt.Println("Read console failed:", err)
				break mainloop
			}

			cmd = strings.TrimSpace(cmd)
			switch cmd {
			case "a":
				status = pos_ALL
			case "n":
				status = pos_NOTEBOOK
			case "q":
				break mainloop
			case "content":
				fmt.Println(ni.Content)
			case "delete":
				fmt.Println("Deleting note.")
				err := yc.DeleteNote(notePath)
				if err != nil {
					fmt.Println("DeleteNote failed:", err)
				}
				status = pos_NOTEBOOK
			default:
				if strings.HasPrefix(cmd, "title ") {
					newTitle := strings.TrimSpace(cmd[len("title "):])
					if len(newTitle) > 0 {
						fmt.Println("Change title to", newTitle)
						err := yc.UpdateNote(notePath, newTitle, ni.Author,
							ni.Source, ni.Content)
						if err != nil {
							fmt.Println("UpdateNote failed:", err)
						}
					}
				} else if strings.HasPrefix(cmd, "author ") {
					newAuthor := strings.TrimSpace(cmd[len("author "):])
					if len(newAuthor) > 0 {
						fmt.Println("Change author to", newAuthor)
						err := yc.UpdateNote(notePath, ni.Title, newAuthor,
							ni.Source, ni.Content)
						if err != nil {
							fmt.Println("UpdateNote failed:", err)
						}
					}
				} else if strings.HasPrefix(cmd, "source ") {
					newSource := strings.TrimSpace(cmd[len("soruce "):])
					if len(newSource) > 0 {
						fmt.Println("Change source to", newSource)
						err := yc.UpdateNote(notePath, ni.Title, ni.Author,
							newSource, ni.Content)
						if err != nil {
							fmt.Println("UpdateNote failed:", err)
						}
					}
				} else if strings.HasPrefix(cmd, "adl ") {
					url := strings.TrimSpace(cmd[len("adl "):])
					if len(url) > 0 {
						fmt.Println(yc.AuthorizeDownloadLink(url))
					}
				}
			}
		}
	}
}
