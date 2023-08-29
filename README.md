# Time Series Data Processor

This is a Go application that processes time series data and saves it to a Parquet file. The application uses a cache to store previously processed data and avoid duplicate writes to the Parquet file.

## File Structure

The file structure of the project is as follows:

```
.
├── cache.go
├── go.mod
├── go.sum
├── main.go
├── README.md
└── time_series_data_processor_test.go
```

- `cache.go`: Contains the implementation of the cache used by the application.
- `go.mod` and `go.sum`: Files used by Go to manage dependencies.
- `main.go`: Contains the main implementation of the application.
- `README.md`: This file.

## Usage

To use the application, you need to set the following environment variables:

- `STORAGE_ACCOUNT_NAME`: The name of the storage account to use for writing the Parquet file. Defaults to "anbofiles".
- `REDIS_HOST`: The hostname of the Redis server to use for caching. Required.
- `REDIS_PORT`: The port number of the Redis server to use for caching. Required.
- `REDIS_PASSWORD`: The password of the Redis server to use for caching. Optional.

Once you have set the environment variables, you can run the application using the following command:

```
go run main.go
```

The application will process the time series data and save it to a Parquet file. The cache will be used to avoid duplicate writes to the Parquet file.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.