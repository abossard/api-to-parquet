package main

import (
	"context"
	md5 "crypto/md5"
	"encoding/hex"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"

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

type TimeSeriesData struct {
	Timestamp       int64   `parquet:"name=Timestamp, type=INT64"`
	TimeOffsetHours int64   `parquet:"name=TimeOffsetHours, type=INT64"`
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

func KeyRequired(keyToCompareWith string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Print("Checking API key")
		suppliedKey := c.Query("key")
		if suppliedKey != keyToCompareWith {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		}
		c.Next()
	}
}

func ReverseProxy(target string) gin.HandlerFunc {
	// extract scheme and host from target variable
	u, err := url.Parse(target)
	if err != nil {
		log.Fatal(err)
	}
	scheme := u.Scheme
	host := u.Host
	path := u.Path
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatal(err)
	}
	return func(c *gin.Context) {
		log.Println("Proxying to " + target)
		token, err := credential.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{"https://api.kusto.windows.net"}})
		if err != nil {
			log.Fatal(err)
		}
		director := func(req *http.Request) {
			req.URL.Scheme = scheme
			req.URL.Host = host
			req.URL.Path = path
			req.Header["Authorization"] = []string{"Bearer " + token.Token} // add the authentication header
		}
		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(c.Writer, c.Request) // forward the request and return the response
	}
}

const lastTimeGeneratedKey = "lastTimestamp"
const maxTimestampKey = "maxTimestamp"

func main() {
	log.Println("Starting API")
	containerName := os.Getenv("STORAGE_CONTAINER_NAME")
	if containerName == "" {
		containerName = "superfiles"
	}
	accountName := os.Getenv("STORAGE_ACCOUNT_NAME")
	if accountName == "" {
		accountName = "anbofiles"
	}
	accountKey := os.Getenv("STORAGE_ACCOUNT_KEY")
	if accountKey == "" {
		accountKey = ""
	}
	requireApiKey := os.Getenv("REQUIRE_API_KEY")
	if requireApiKey == "" {
		requireApiKey = ""
	}
	omitStartUpCheck := os.Getenv("OMIT_STARTUP_CHECK")
	if omitStartUpCheck == "" {
		omitStartUpCheck = ""
	}

	postBackendUrl := os.Getenv("POST_BACKEND_URL")
	if postBackendUrl == "" {
		postBackendUrl = "https://adxpoolhisv2.synapse-jsontoparquet-hisv2.kusto.azuresynapse.net/v2/rest/query"
	}

	cache, err := NewCache()
	log.Println("Starting cache....")
	if err != nil {
		log.Fatal(err)
	}
	// put the maximum duration of 100 years
	years100 := time.Duration(100 * 365 * 24 * time.Hour)

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

	if omitStartUpCheck == "" {
		cache.Set("foo", "bar", years100)
		var value string
		cache.Get("foo", &value)
		if value != "bar" {
			log.Fatal("value not found, cache not working")
		}

		var data []TimeSeriesData
		for i := 0; i < 1000; i++ {
			data = append(data, TimeSeriesData{
				Timestamp:       time.Now().Unix(),
				TimeOffsetHours: 0,
				PointId:         "PointId",
				Sequence:        0,
				Project:         "Project",
				Res:             "Res",
				Quality:         0,
				Value:           float64(i),
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
		response, err := blobClient.UploadFile(context.TODO(), containerName, "startup_test.parquet", tmpFile, nil)
		if err != nil {
			log.Fatal(err)
		}
		log.Print(response)
	}
	router := gin.Default()
	router.Use(gin.Logger())

	if requireApiKey != "" {
		log.Println("Using API key")
		router.Use(KeyRequired(requireApiKey))
	} else {
		log.Println("No API key specified with REQUIRE_API_KEY environment variable, not using one.")
	}

	router.GET("/", func(c *gin.Context) {
		var lastTimeGenerated int64
		var maxTimestamp int64

		cache.Get(lastTimeGeneratedKey, &lastTimeGenerated)
		cache.Get(maxTimestampKey, &maxTimestamp)

		c.IndentedJSON(200, gin.H{
			"lastTimeGenerated": lastTimeGenerated,
			"maxTimestamp":      maxTimestamp,
		})
	})

	router.POST("/query", ReverseProxy(postBackendUrl))
	router.POST("/", func(c *gin.Context) {
		log.Printf("Request information: %+v", c.Request)
		var newRecord input_record

		if err := c.BindJSON(&newRecord); err != nil {
			c.AbortWithError(500, err)
			return
		}
		if newRecord.File == "" {
			c.JSON(400, gin.H{"error": "Malformed request: property file is empty"})
			return
		}

		if newRecord.TimeGenerated == 0 {
			c.JSON(400, gin.H{"error": "Malformed request: property timeGenerated is empty"})
			return
		}

		if newRecord.Id == "" {
			c.JSON(400, gin.H{"error": "Malformed request: property id is empty"})
			return
		}

		if len(newRecord.Content) > 0 {
			log.Printf("First time series data point: %+v", newRecord.Content[0])
		} else {
			log.Printf("No time series data points")
		}

		log.Printf("New record statistics: entries=%d, first_timestamp=%v, last_timestamp=%v, file=%s, time_generated=%v",
			len(newRecord.Content), newRecord.Content[0].Timestamp, newRecord.Content[len(newRecord.Content)-1].Timestamp, newRecord.File, newRecord.TimeGenerated)

		var maxTimestamp int64
		for _, v := range newRecord.Content {
			if v.Timestamp > maxTimestamp {
				maxTimestamp = v.Timestamp
			}
		}
		log.Printf("Max timestamp: %v", maxTimestamp)

		hasher := md5.New()
		hasher.Write([]byte(newRecord.File))
		hashedFileName := hex.EncodeToString(hasher.Sum(nil))

		tmpFile, err := os.CreateTemp(os.TempDir(), hashedFileName)
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
		log.Printf("Uploaded file to blob storage. With Request ID: %+v", response.RequestID)

		cache.Set(lastTimeGeneratedKey, newRecord.TimeGenerated, years100)

		var currentMaxTimestamp int64
		cache.Get(maxTimestampKey, &currentMaxTimestamp)
		if maxTimestamp > currentMaxTimestamp {
			log.Printf("Updating maxTimestamp from %v to %v", currentMaxTimestamp, maxTimestamp)
			cache.Set(maxTimestampKey, maxTimestamp, years100)
		} else {
			log.Printf("Not updating maxTimestamp, current value is %v", currentMaxTimestamp)
		}

		c.IndentedJSON(200, gin.H{
			"id":            newRecord.Id,
			"timeGenerated": newRecord.TimeGenerated,
			"maxTimestamp":  maxTimestamp,
		})
	})
	router.Run(":8080")
}
