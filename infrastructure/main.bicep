param salt string = uniqueString(resourceGroup().id)

param projectName string = 'jsontoparquet'

param containerAppName string = 'ca-${projectName}-${salt}'
param containerRegistryName string = 'acr${salt}'
param containerAppEnvName string = 'caenv-${projectName}-${salt}'
param imageWithTag string = 'js2par:latest'
param location string = resourceGroup().location

param containerAppLogAnalyticsName string = 'calog-${projectName}-${salt}'
param storageAccountName string = 'castrg${salt}'
param blobContainerName string = 'parquet${salt}'

var acrPullRole = resourceId('Microsoft.Authorization/roleDefinitions', '7f951dda-4ed3-4680-a7ca-43fe172d538d')

var storageRole = resourceId('Microsoft.Authorization/roleDefinitions', 'ba92f5b4-2d11-453d-a403-e96b0029c9fe')

resource acr 'Microsoft.ContainerRegistry/registries@2021-06-01-preview' = {
  name: containerRegistryName
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    adminUserEnabled: true
  }
}

resource uai 'Microsoft.ManagedIdentity/userAssignedIdentities@2022-01-31-preview' = {
  name: 'id-${containerAppName}'
  location: location
}


resource uaiRbac 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(resourceGroup().id, uai.id, acrPullRole)
  scope: acr
  properties: {
    roleDefinitionId: acrPullRole
    principalId: uai.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

resource uaiRbacStorage 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(resourceGroup().id, uai.id, storageRole)
  scope: sa
  properties: {
    roleDefinitionId: storageRole
    principalId: uai.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

resource sa 'Microsoft.Storage/storageAccounts@2021-04-01' = {
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

resource fileServices 'Microsoft.Storage/storageAccounts/fileServices@2021-04-01' = {
  parent: sa
  name: 'default'
}
resource redisShare 'Microsoft.Storage/storageAccounts/fileServices/shares@2021-04-01' = {
  parent: fileServices
  name: 'redis'
}

resource blobContainer 'Microsoft.Storage/storageAccounts/blobServices/containers@2022-09-01' = {
  name: blobContainerName
  parent: blobServices
  properties: {
  }
}

resource logAnalytics 'Microsoft.OperationalInsights/workspaces@2021-06-01' = {
  name: containerAppLogAnalyticsName
  location: location
  properties: {
    sku: {
      name: 'PerGB2018'
    }
  }
}

resource containerAppEnv 'Microsoft.App/managedEnvironments@2022-01-01-preview' = {
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

resource redisCaEnvStorage 'Microsoft.App/managedEnvironments/storages@2023-04-01-preview' = {
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
      registries: [
        {
          identity: uai.id
          server: acr.properties.loginServer
        }
      ]
      ingress: {
        external: true
        targetPort: 8080
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
          image: '${acr.properties.loginServer}/${imageWithTag}'
          env: [
            {
              name: 'STORAGE_ACCOUNT_NAME'
              value: sa.name
            }
            {
              name: 'STORAGE_CONTAINER_NAME'
              value: blobContainer.name
            }
            {
              name: 'AZURE_CLIENT_ID'
              value: uai.properties.clientId
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
        maxReplicas: 1
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
