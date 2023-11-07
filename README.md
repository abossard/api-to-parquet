# Time Series Data Processor

This is a Go application that processes time series data and saves it to a Parquet file. The application uses a cache to store previously processed data and avoid duplicate writes to the Parquet file.

## Roadmap
[x] Secret token concept for the API, e.g. REQUIRE_API_KEY=abcdefghs
[x] Data verfication method

## Deployment instructions
[![Deploy To Azure](https://raw.githubusercontent.com/Azure/azure-quickstart-templates/master/1-CONTRIBUTION-GUIDE/images/deploytoazure.svg?sanitize=true)](https://portal.azure.com/#create/Microsoft.Template/uri/https%3A%2F%2Fraw.githubusercontent.com%2Fabossard%2Fapi-to-parquet%2Fmain%2Finfrastructure%2Fazuredeploy.json)

[![Visualize](https://raw.githubusercontent.com/Azure/azure-quickstart-templates/master/1-CONTRIBUTION-GUIDE/images/visualizebutton.svg?sanitize=true)](http://armviz.io/#/?load=https%3A%2F%2Fraw.githubusercontent.com%2Fabossard%2Fapi-to-parquet%2Fmain%2Finfrastructure%2Fazuredeploy.json)

After the deployment finished, you'll see the public URL with the API Key in the deployment output. Also the static IP of the container app environmnet.

For more detailed instructions and other scenarios check: [setup.azcli](infrastructure/setup.azcli) 

## Shared Access Signature (SAS) Token
By default, the deployment adds an api key or shared access signature to protect the API. After the deployment you'll get the key in the `apiKey` output. 

To use the key during the call, add a query parameter `key` to the url, e.g.:

```
GET https://api.mydomain.com/?key=abcdef0123456789 HTTP/1.1
```

## GET: How to get the lastTimeGenerated and maxTimestamp

```
GET https://api.mydomain.com/?key=abcdef0123456789 HTTP/1.1
```

Will return the `lastTimeGenerated` property as well as the `maxTimestamp`, which is the highest timestamp
that has even been sent to the API.

## POST: How to upload new data

(the format of the post body is meant to be executed e.g. with HTTP Client in VS Code.)

```
POST https://api.mydomain.com/?key=abcdef0123456789 HTTP/1.1
content-type: application/json

{
    "content": [
        {
            "timestamp": {{$timestamp}},
            "value": {{$randomInt 1 43}},
            "timeOffsetHours": {{$randomInt 1 43}},
            "pointId": "{{$guid}}",
            "sequence": {{$randomInt 1 43}},
            "project": "{{$guid}}",
            "res": "{{$guid}}",
            "quality": {{$randomInt 1 43}}
        },
        {
            "timestamp": {{$timestamp}},
            "value": {{$randomInt 1 43}},
            "timeOffsetHours": {{$randomInt 1 43}},
            "pointId": "{{$guid}}",
            "sequence": {{$randomInt 1 43}},
            "project": "{{$guid}}",
            "res": "{{$guid}}",
            "quality": {{$randomInt 1 43}}
        },
        {
            "timestamp": {{$timestamp}},
            "value": {{$randomInt 1 43}},
            "timeOffsetHours": {{$randomInt 1 43}},
            "pointId": "{{$guid}}",
            "sequence": {{$randomInt 1 43}},
            "project": "{{$guid}}",
            "res": "{{$guid}}",
            "quality": {{$randomInt 1 43}}
        }
    ],
    "file": "2023/10/11/{{$timestamp}}-{{$guid}}.parquet",
    "timeGenerated": {{$timestamp}},
    "id": "{{$guid}}"
}
```
- `content`: Contains the data that is being written to parquet
- `file`: is the filename that is used when the output blob is created. If there is an existing blob, it will be overwritten with the new data
- `timeGenerated`: is being used as the `lastTimeGenerated` when the `/` is being called
- `id`: Doesn't have a purpose yet

## Examples Synapse Query
```
SELECT TOP 100 *
FROM
    OPENROWSET(
        BULK 'https://ACCOUNTNAME.blob.core.windows.net/CONTAINERNAME/tests/2023/10/26/19/*.parquet',
        FORMAT='PARQUET'
    ) AS data
```

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
- `REQUIRE_API_KEY`: The API key that must be provided in the request header as `key` in the URL
- `REDIS_HOST`: The hostname of the Redis server to use for caching. Required.
- `REDIS_PORT`: The port number of the Redis server to use for caching. Required.
- `REDIS_PASSWORD`: The password of the Redis server to use for caching. Optional.

Once you have set the environment variables, you can run the application using the following command:

```
cd src
go run .
```

The application will process the time series data and save it to a Parquet file. The cache will be used to avoid duplicate writes to the Parquet file.

## API Security
It's possible to configure the API to require a secret token to be provided in the request header as `key` in the URL. This is done by setting the environment variable `REQUIRE_API_KEY` to a secret token. If this environment variable is not set, the API will not require a secret token.

On top of that you can always enable OIDC on the Azure Container App level.

## Authentication to Azure Storage

The application uses the Azure SDK for Go to authenticate with Azure Storage. The SDK uses the Azure Default Credential Provider Chain to authenticate. This means e.g. locally it will use the Azure CLI to authenticate, and in Azure it will use Managed Identity. Please see the [Azure SDK for Go documentation](https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authorization) for more information.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
