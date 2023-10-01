param salt string = uniqueString(resourceGroup().id)

param projectName string = 'jsontoparquet'

param containerAppName string = 'ca-${projectName}-${salt}'
param containerRegistryName string = 'acr${salt}'
param containerAppEnvName string = 'caenv-${projectName}-${salt}'
param imageWithTag string = 'js2par:latest'
param onlyDeployNginxExample bool = false
param location string = resourceGroup().location
param useManagedIdentity bool = false
param redisCacheName string = 'redis-${projectName}-${salt}'
param containerAppLogAnalyticsName string = 'calog-${projectName}-${salt}'
param storageAccountName string = 'castrg${salt}'
param blobContainerName string = 'parquet${salt}'

var acrPullRole = resourceId('Microsoft.Authorization/roleDefinitions', '7f951dda-4ed3-4680-a7ca-43fe172d538d')
var storageRole = resourceId('Microsoft.Authorization/roleDefinitions', 'ba92f5b4-2d11-453d-a403-e96b0029c9fe')

resource acr 'Microsoft.ContainerRegistry/registries@2023-07-01' = {
  name: containerRegistryName
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    adminUserEnabled: true
  }
}

resource uai 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' = {
  name: 'id-${containerAppName}'
  location: location
}


resource redisCache 'Microsoft.Cache/Redis@2020-06-01' = {
  name: redisCacheName
  location: location
  properties: {
    enableNonSslPort: false
    minimumTlsVersion: '1.2'
    sku: {
      capacity: 1
      family: 'C'
      name: 'Basic'
    }
  }
}


resource uaiRbac 'Microsoft.Authorization/roleAssignments@2022-04-01' = if(useManagedIdentity) {
  name: guid(resourceGroup().id, uai.id, acrPullRole)
  scope: acr
  properties: {
    roleDefinitionId: acrPullRole
    principalId: uai.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

resource uaiRbacStorage 'Microsoft.Authorization/roleAssignments@2022-04-01' = if(useManagedIdentity) {
  name: guid(resourceGroup().id, uai.id, storageRole)
  scope: sa
  properties: {
    roleDefinitionId: storageRole
    principalId: uai.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

resource sa 'Microsoft.Storage/storageAccounts@2023-01-01' = {
  name: storageAccountName
  location: location
  kind: 'StorageV2'
  sku: {
    name: 'Standard_LRS'
  }
  properties: {
    isHnsEnabled: true
    accessTier: 'Hot'
  }
}
resource blobServices 'Microsoft.Storage/storageAccounts/blobServices@2023-01-01' = {
  parent: sa
  name: 'default'
}

resource fileServices 'Microsoft.Storage/storageAccounts/fileServices@2023-01-01' = {
  parent: sa
  name: 'default'
}
resource redisShare 'Microsoft.Storage/storageAccounts/fileServices/shares@2023-01-01' = {
  parent: fileServices
  name: 'redis'
}

resource blobContainer 'Microsoft.Storage/storageAccounts/blobServices/containers@2023-01-01' = {
  name: blobContainerName
  parent: blobServices
  properties: {
  }
}

resource logAnalytics 'Microsoft.OperationalInsights/workspaces@2022-10-01' = {
  name: containerAppLogAnalyticsName
  location: location
  properties: {
    sku: {
      name: 'PerGB2018'
    }
  }
}

resource containerAppEnv 'Microsoft.App/managedEnvironments@2023-05-01' = {
  name: containerAppEnvName
  location: location
  properties: {
    appLogsConfiguration: {
      destination: 'log-analytics'
      logAnalyticsConfiguration: {
        customerId: logAnalytics.properties.customerId
        sharedKey: logAnalytics.listKeys().primarySharedKey
      }
    }
  }
}

resource redisCaEnvStorage 'Microsoft.App/managedEnvironments/storages@2023-05-01' = {
  parent: containerAppEnv
  name: redisShare.name
  properties: {
    azureFile: {
      accountName: sa.name
      shareName: redisShare.name
      accessMode: 'ReadWrite'
      accountKey: sa.listKeys().keys[0].value
    }
  }
}

resource containerApp 'Microsoft.App/containerApps@2023-05-01' = {
  name: containerAppName
  location: location
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: {
      '${uai.id}': {}
    }
  }
  properties: {
    managedEnvironmentId: containerAppEnv.id
    configuration: {
      secrets: [
        {
          name: 'myregistrypassword'
          value:  acr.listCredentials().passwords[0].value
        }
        {
          name: 'storageaccountkey'
          value: sa.listKeys().keys[0].value
        }
      ]
      registries: [ 
        {
          server: acr.properties.loginServer
          username: acr.listCredentials().username
          passwordSecretRef: 'myregistrypassword'
        }
      ]
      ingress: {
        external: true
        targetPort: onlyDeployNginxExample ? 80: 8080
        allowInsecure: false
        traffic: [
          {
            latestRevision: true
            weight: 100
          }
        ]
      }
    }
    template: {
      containers: [
        {
          name: containerAppName
          image: onlyDeployNginxExample ? 'nginx' : '${acr.properties.loginServer}/${imageWithTag}' 
          env: [
            {
              name: 'STORAGE_ACCOUNT_KEY'
              secretRef: 'storageaccountkey'
            }
            {
              name: 'AZURE_CLIENT_ID'
              value: uai.properties.clientId
            }
            {
              name: 'STORAGE_ACCOUNT_NAME'
              value: sa.name
            }
            {
              name: 'STORAGE_CONTAINER_NAME'
              value: blobContainer.name
            }
            {
              name: 'REDIS_HOST'
              value: redisCache.properties.hostName
            }
            {
              name: 'REDIS_PORT'
              value: '6380'
            }
            {
              name: 'REDIS_PASSWORD'
              value: redisCache.listKeys().primaryKey
            }
          ]
          resources: {
            cpu: json('1')
            memory: '2Gi'
          }
        }
      ]
      scale: {
        minReplicas: 1
        maxReplicas: 10
        rules: [
          {
            name: 'http-requests'
            http: {
              metadata: {
                concurrentRequests: '10'
              }
            }
          }
        ]
      }
    }
  }
}

output containerAppFQDN string = containerApp.properties.configuration.ingress.fqdn
