package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	_ "github.com/go-sql-driver/mysql"
	"database/sql"
	"golang.org/x/net/html"
	im "github.com/hristiyangoranov/go-webscraper/imageclass"
	"html/template"
)

var db, _ =sql.Open("mysql", "root:gocoursepass@tcp(127.0.0.1:3306)/Homework")
var MaxDepth = 2
var wg sync.WaitGroup
var linksresult []Link
var visited map[string]bool
var tmpl =template.New("template")

type Link struct{
	text 	string
	depth 	int
	url 	string
}

func(self Link) Valid() bool{
	if self.depth>=MaxDepth{
		return false
	}
	if len(self.text)==0{
		return false
	}
	if len(self.url) == 0{
		return false
	}
	return true
}

//returns a slice of links gathered from a given html file
func LinkReader(r *http.Response, depth int, visited map[string]bool) []Link{
	page := html.NewTokenizer(r.Body)
	var links []Link

	var start *html.Token
	var text string
	for {
		page.Next()
		token:=page.Token()
		if token.Type==html.ErrorToken{
			break
		}
		if start!=nil && token.Type==html.TextToken{
			text=fmt.Sprintf("%s%s", text, token.Data)
		}
		switch token.Type{
		case html.StartTagToken:
			if len(token.Attr) > 0 {
				start = &token
			}
		case html.EndTagToken:
			if start == nil {
				continue
			}
			link := NewLink(*start, text, depth)
			if link.Valid() {
				//fmt.Printf("Link Found %v", link.url)
				links = append(links, link)
			}

			start = nil
			text = ""
			
		}
		
	}
	return links
}

func NewLink(tag html.Token, text string, depth int) Link{
	link:=Link{text:strings.TrimSpace(text),depth:depth}
	for i:= range tag.Attr{
		if tag.Attr[i].Key=="href"{
			link.url=strings.TrimSpace(tag.Attr[i].Val)
		}
	}
	if visited[link.url]{
		link.text=""
		link.url=""
	}
	return link
}

func GetLinksFromURL(url string, depth int, linkschan chan Link, visited map[string]bool){
	currWorkers-=1
	defer wg.Done()
	page, err := GetResponseFromURL(url)
	if err != nil {
		//fmt.Print(err)
		return 
	}
	links := LinkReader(page, depth, visited)
	if depth>MaxDepth{
		return 
	}
	for i:=range links{
		linkschan<-links[i]
	}
	return 
}


func GetResponseFromURL(url string) (resp *http.Response, err error) {
	resp, err = http.Get(url)
	if err != nil {
		//fmt.Printf("Error: %s \n", err)
		return
	}

	if resp.StatusCode > 299 {
		return
	}
	return

}
var maxWorkers = 10
var currWorkers int
func start(links []Link, depth int, linkschan chan Link, imagechan chan im.Image, visited map[string]bool){
	if depth>MaxDepth{
		return
	}
	for i := range links{
		currWorkers+=2
		if currWorkers>maxWorkers{
			wg.Wait()
		}
		wg.Add(2)
		go GetLinksFromURL(links[i].url, depth+1, linkschan, visited)
		go downloadAllImagesFromURL(links[i].url, imagechan )
	}
	//start (linksresult, depth+1, linkschan)
}
func downloadImageFromURL(imageURL string, imagechan chan im.Image, alttext string) error {
	var image im.Image
	resp, err:=http.Get(imageURL)
	if err!=nil{
		return err
	}
	segments:=strings.Split(imageURL, "/")
	lastsegment:=segments[len(segments)-1]
	image.Filename=strings.Split(lastsegment, "?")[0]
	image.ImageURL=imageURL
	if(len(strings.Split(image.Filename, "."))>1){
		image.Format=strings.Split(image.Filename,".")[1]
	}
	image.AltText=alttext

	file, err:=os.Create(image.Filename)
	if err!=nil{
		return err
	}
	defer file.Close()
	io.Copy(file,resp.Body)
	imagechan<-image
	return nil
}
func downloadAllImagesFromURL(URL string, imagechan chan im.Image) error{

	defer wg.Done()
	currWorkers-=1
	resp, err :=GetResponseFromURL(URL)
	if err!=nil{
		return err
	}
	page:=html.NewTokenizer(resp.Body)

	for {
		page.Next()
		token:=page.Token()
		if token.Type==html.ErrorToken{
			break
		}
		if token.Data=="img"{
			alttext:=getAlttext(token)
			for i:=0;i<len(token.Attr); i++{
				if token.Attr[i].Key=="src"{
					imageURL:=token.Attr[i].Val
					downloadImageFromURL(imageURL, imagechan, alttext)
					break
				}
			}
		}

	}
	return nil
}
func getAlttext(token html.Token)string{
	for i:=0;i<len(token.Attr);i++{
		if token.Attr[i].Key=="alt"{
			return token.Attr[i].Val
		}
	}
	return ""
}
func HandleSearch(w http.ResponseWriter, r *http.Request){
	tmpl.ExecuteTemplate(w,"search.html" ,nil)
}

func Resulthandler(w http.ResponseWriter, r *http.Request){
		format := r.URL.Query().Get("format")
		altText := r.URL.Query().Get("altText")
		querry:="SELECT * FROM Homework.ImageMetadata WHERE format=? AND alternativeText=?;"
		rows, err:=db.Query(querry, format, altText)
		if err!=nil{
			fmt.Print(err)
			return
		}
		defer rows.Close()
		var url string
		var filename string
		var picture []byte
		rows.Next()
		fmt.Print(rows.Scan(&url, &filename, &format, &altText, &picture))

		w.Write(picture)
		//tmpl.ExecuteTemplate(w, "res.html", filename)
	
}
func main(){
	defer db.Close()
	//read from the channel
	wg.Add(1)
	imagechan:=make(chan im.Image)
	linkschan:=make(chan Link)
	go func(linkschan chan Link){
		for{
			select {
			case msg:= <-linkschan:
				linksresult=append(linksresult, msg)
				fmt.Print(msg.url)
				fmt.Print("\n")
			}
		}
	}(linkschan)
	go func (imagechan chan im.Image){
		for{
			select{
			case image:=<-imagechan:
				im.InsertIntoDB(image, db)
			}
		}
	}(imagechan)
	GetLinksFromURL(os.Args[1], 0, linkschan, visited)
	start(linksresult, 0, linkschan, imagechan, visited)
	wg.Wait()
	tmpl, _=template.ParseGlob("*.html")
	http.HandleFunc("/", HandleSearch)
	http.HandleFunc("/result", Resulthandler)
	fmt.Print("listening")
	http.ListenAndServe(":8080", nil)
}