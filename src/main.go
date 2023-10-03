package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	delta "github.com/csimplestring/delta-go"
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
	Timestamp       int64   `parquet:"name=Timestamp, type=INT64"`
	TimeOffsetHours int8    `parquet:"name=TimeOffsetHours, type=INT64"`
	PointId         string  `parquet:"name=PointId,  type=BYTE_ARRAY, convertedtype=UTF8"`
	Sequence        int32   `parquet:"name=Sequence, type=INT64"`
	Project         string  `parquet:"name=Project,  type=BYTE_ARRAY, convertedtype=UTF8"`
	Value           float64 `parquet:"name=Value, type=DOUBLE"`
	Res             string  `parquet:"name=Res,  type=BYTE_ARRAY, convertedtype=UTF8"`
	Quality         int32   `parquet:"name=Quality, type=INT64"`
}

type input_record struct {
	Content       []TimeSeriesData `json:"content"`
	Id            string           `json:"id"`
	Source        string           `json:"source"`
	TimeGenerated int64            `json:"timeGenerated"`
	File          string           `json:"file"`
}

const lastTimeGeneratedKey = "lastTimestamp"

func main() {
	log.Println("Starting API")
	containerName := os.Getenv("STORAGE_CONTAINER_NAME")
	if containerName == "" {
		containerName = "superfiles"
	}
	dfsEndpoint := os.Getenv("DATALAKE_ENDPOINT")
	if dfsEndpoint == "" {
		dfsEndpoint = "https://castrgqmh6n6ngd4pj4.dfs.core.windows.net/"
	}
	accountName := os.Getenv("STORAGE_ACCOUNT_NAME")
	if accountName == "" {
		accountName = "anbofiles"
	}
	accountKey := os.Getenv("STORAGE_ACCOUNT_KEY")
	if accountKey == "" {
		accountKey = ""
	}

	format := os.Getenv("FORMAT")
	if format == "" {
		format = "PARQUET"
	}

	log.Println(accountName)

	cache, err := NewCache()
	log.Println("Starting cache....")
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

	var blobClient *azblob.Client

	if accountKey == "" {
		log.Println("No account key, using default credentials")
		credential, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			log.Fatal(err)
		}
		blobClient, _ = azblob.NewClient("https://"+accountName+".blob.core.windows.net/", credential, nil)
	} else {
		log.Println("Using account key")
		credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
		if err != nil {
			log.Fatal(err)
		}
		blobClient, _ = azblob.NewClientWithSharedKeyCredential("https://"+accountName+".blob.core.windows.net/", credential, nil)
	}

	if err != nil {
		log.Fatal(err)
	}
	log.Print("Got Azure Credentials")
	log.Print("Got Blob Client")
	// TEST Delta Lake API

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
		saveTimeseriesToBlobStorage(newRecord, containerName, blobClient)

		cache.Set(lastTimeGeneratedKey, newRecord.TimeGenerated, years100)

		c.IndentedJSON(200, gin.H{
			"id":            newRecord.Id,
			"timeGenerated": newRecord.TimeGenerated,
		})
	})
	router.Run(":8080")
}

func saveTimeseriesToBlobStorage(newRecord input_record, containerName string, blobClient *azblob.Client) error {
	// Create a temporary file to store the timeseries data
	tmpFile, err := os.CreateTemp(os.TempDir(), newRecord.File)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	// Save the timeseries data to a Parquet file
	if err := saveTimeseriesToParquetFile(tmpFile, newRecord.Content); err != nil {
		return err
	}
	tmpFile.Close()

	// Reopen the temporary file
	tmpFile, err = os.Open(tmpFile.Name())
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	// Upload the Parquet file to Azure Blob Storage
	response, err := blobClient.UploadFile(context.TODO(), containerName, newRecord.File, tmpFile, nil)
	if err != nil {
		return err
	}
	log.Print(response)

	return nil
}
