package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/gin-gonic/gin"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

func saveTimeseriesToParquetFile(file *os.File, data []TimeSeriesData) error {

	var err error

	pw, err := writer.NewParquetWriterFromWriter(file, new(TimeSeriesData), 4)
	if err != nil {
		log.Println("Can't create parquet writer", err)
		return err
	}

	pw.RowGroupSize = 128 * 1024 * 1024 //128M
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	for _, v := range data {
		if err = pw.Write(v); err != nil {
			log.Println("Write error", err)
			return err
		}
	}

	if err = pw.WriteStop(); err != nil {
		log.Println("WriteStop error", err)
		return err
	}
	stats, err := file.Stat()
	if err != nil {
		log.Println("Can't get file stats", err)
		return err
	}

	log.Println("Write Finished to local file: " + file.Name())
	log.Printf("File Size: %d bytes", stats.Size())
	return nil
}

// album represents data about a record album.
type TimeSeriesData struct {
	Timestamp int64   `parquet:"name=Timestamp, type=INT32"`
	Value     float64 `parquet:"name=Value, type=DOUBLE"`
}

type input_record struct {
	Content       []TimeSeriesData `json:"content"`
	Id            string           `json:"id"`
	TimeGenerated int64            `json:"timeGenerated"`
	File          string           `json:"file"`
}

const lastTimeGeneratedKey = "lastTimestamp"

func main() {
	containerName := os.Getenv("STORAGE_CONTAINER_NAME")
	if containerName == "" {
		containerName = "superfiles"
	}
	accountName := os.Getenv("STORAGE_ACCOUNT_NAME")
	if accountName == "" {
		accountName = "anbofiles"
	}
	log.Println(accountName)

	cache, err := NewCache()
	if err != nil {
		log.Fatal(err)
	}
	// put the maximum duration of 100 years
	years100 := time.Duration(100 * 365 * 24 * time.Hour)

	cache.Set("foo", "bar", years100)
	var value string
	cache.Get("foo", &value)
	if value != "bar" {
		log.Fatal("value not found, cache not working")
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Got Azure Credentials")

	blobClient, _ := azblob.NewClient("https://"+accountName+".blob.core.windows.net/", cred, nil)

	log.Print("Got Blob Client")
	// generate a TimeseriesData with a for loop
	var data []TimeSeriesData
	for i := 0; i < 1000; i++ {
		data = append(data, TimeSeriesData{
			Timestamp: time.Now().Unix(),
			Value:     float64(i),
		})
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "myfile")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	saveTimeseriesToParquetFile(tmpFile, data)
	tmpFile.Close()

	// reopen tmpFile
	tmpFile, err = os.Open(tmpFile.Name())
	if err != nil {
		log.Fatal(err)
	}
	defer tmpFile.Close()
	response, err := blobClient.UploadFile(context.TODO(), containerName, "myfile2.parquet", tmpFile, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Print(response)

	router := gin.Default()
	router.Use(gin.Logger())
	router.GET("/", func(c *gin.Context) {
		var lastTimeGenerated int64
		cache.Get(lastTimeGeneratedKey, &lastTimeGenerated)

		c.IndentedJSON(200, gin.H{
			"lastTimeGenerated": lastTimeGenerated,
		})
	})
	router.POST("/", func(c *gin.Context) {
		var newRecord input_record

		if err := c.BindJSON(&newRecord); err != nil {
			c.AbortWithError(500, err)
			return
		}

		tmpFile, err := os.CreateTemp(os.TempDir(), newRecord.File)
		if err != nil {
			log.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())
		saveTimeseriesToParquetFile(tmpFile, newRecord.Content)
		tmpFile.Close()

		// reopen tmpFile
		tmpFile, err = os.Open(tmpFile.Name())
		if err != nil {
			log.Fatal(err)
		}
		defer tmpFile.Close()
		response, err := blobClient.UploadFile(context.TODO(), containerName, newRecord.File, tmpFile, nil)
		if err != nil {
			log.Fatal(err)
		}
		log.Print(response)

		cache.Set(lastTimeGeneratedKey, newRecord.TimeGenerated, years100)

		c.IndentedJSON(200, gin.H{
			"id":            newRecord.Id,
			"timeGenerated": newRecord.TimeGenerated,
		})

		// fw, err := local.NewLocalFileWriter(newRecord.File)
		// if err != nil {
		// 	c.AbortWithError(502, err)
		// 	return
		// }
		// defer fw.Close()

		// pw, err := writer.NewParquetWriter(fw, new(TimeSeriesData), 4)
		// if err != nil {
		// 	c.AbortWithError(503, err)
		// 	return
		// }
		// pw.RowGroupSize = 128 * 1024 * 1024 //128M
		// pw.PageSize = 8 * 1024              //8K
		// pw.CompressionType = parquet.CompressionCodec_SNAPPY
		// num := 100
		// for i := 0; i < num; i++ {
		// 	stu := TimeSeriesData{
		// 		Timestamp: time.Now(),
		// 		Value:     float64(i),
		// 	}
		// 	if err = pw.Write(stu); err != nil {
		// 		log.Println("Write error", err)
		// 	}
		// }
		// // for _, v := range newRecord.Content {
		// // 	if err = pw.Write(v); err != nil {
		// // 		c.AbortWithError(504, err)
		// // 		return
		// // 	}
		// // }
		// if err = pw.WriteStop(); err != nil {
		// 	log.Println("WriteStop error", err)
		// }
		// fw.Close()

		// // Open the file to upload
		// fileHandler, err := os.Open(newRecord.File)

		// // close the file after it is no longer required.
		// // defer func(file *os.File) {
		// // 	err = file.Close()
		// // 	if err != nil {
		// // 		c.AbortWithError(503, err)
		// // 		return
		// // 	}
		// // }(fileHandler)

		// // delete the local file if required.
		// // defer func(name string) {
		// // 	err = os.Remove(name)
		// // 	if err != nil {
		// // 		c.AbortWithError(503, err)
		// // 		return
		// // 	}
		// // }(newRecord.File)

		// fileStats, err := fileHandler.Stat()
		// log.Println(fileStats.Size())
		// // store file in blob storage
		// uploadResp, err := blobClient.UploadFile(c,
		// 	containerName,
		// 	newRecord.File,
		// 	fileHandler,
		// 	&azblob.UploadFileOptions{
		// 		BlockSize:   int64(1024),
		// 		Concurrency: uint16(3),
		// 		// If Progress is non-nil, this function is called periodically as bytes are uploaded.
		// 		Progress: func(bytesTransferred int64) {
		// 			fmt.Println(bytesTransferred)
		// 		},
		// 	})

		// if err != nil {
		// 	c.AbortWithError(504, err)
		// 	return
		// }

		// c.IndentedJSON(http.StatusCreated, uploadResp)
	})
	router.Run("localhost:8080")
}
