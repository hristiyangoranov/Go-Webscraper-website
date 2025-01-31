package imageclass

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	_ "github.com/go-sql-driver/mysql"
)
type Image struct{
	ImageURL string
	AltText string
	Format string
	Filename string
}
func InsertIntoDB(Image Image, db *sql.DB) {
	

	imagePath := Image.Filename
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read image file: %v", err)
	}

	query:="INSERT INTO ImageMetadata (url, filename, format, alternativeText, thumbnail) VALUES (?, ?, ?, ?, ?)"
	_, err = db.Exec(query, Image.ImageURL, Image.Filename, Image.Format, Image.AltText, imageData)
	if err != nil {
		log.Fatalf("Failed to insert image into database: %v", err)
	}

	fmt.Println("Image inserted successfully!")
}
