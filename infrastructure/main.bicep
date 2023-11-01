@description('A randrom unique string to salt all names.')
param salt string = substring(uniqueString(resourceGroup().id), 0, 5)

@description('The name of the project. Used to generate names.')
param projectName string = 'jsontoparquet'

@description('The image to use for the container app.')
param imageWithTag string = 'js2par:latest'

@description('The location to deploy to.')
param location string = resourceGroup().location

@description('The environment to deploy to.')
param useManagedIdentity bool = false

@description('Should it deploy Synapse?')
param deploySynapse bool = true

@description('Should it deploy the container app?')
param deployApps bool = true

@description('The API key to use for the container app. Or empty for none.')
param apiKeyToUse string = uniqueString(resourceGroup().id, deployment().name)

@description('Should it enable IP whitelisting?')
param enableIpWhitelist bool = true

@description('The IP whitelist. Comma separated list of IPs.')
param ipWhitelist string = ''

@description('The address space for the vnet')
param vnetAddressSpace string = '10.144.0.0/20'

@description('Build container app image?')
param doBuildContainerAppImage bool = true

// some default names
param containerAppName string = 'ca-${projectName}-${salt}'
param containerRegistryName string = 'acr${salt}'
param containerAppEnvName string = 'caenvvnet-${projectName}-${salt}'
param redisCacheName string = 'redis-${projectName}-${salt}'
param containerAppLogAnalyticsName string = 'calog-${projectName}-${salt}'
param storageAccountName string = 'castrg${salt}'
param blobContainerName string = 'parquet${salt}'
param synapseWorkspaceName string = 'synapse-${projectName}-${salt}'
param vnetName string = 'vnet${projectName}${salt}'
param adxPoolName string = 'adxpool${salt}'
param adxDatabaseName string = 'adxdb${salt}'
param githubApiRepositoryUrl string = 'https://github.com/abossard/api-to-parquet.git'
param githubApiRepositoryBranch string = 'main'

var acrPullRole = resourceId('Microsoft.Authorization/roleDefinitions', '7f951dda-4ed3-4680-a7ca-43fe172d538d')
var storageRole = resourceId('Microsoft.Authorization/roleDefinitions', 'ba92f5b4-2d11-453d-a403-e96b0029c9fe')

param containerAppSubnetName string = 'containerapp'

var appEnvSubnetCidr = cidrSubnet(vnetAddressSpace, 23, 0)

module buildContainerImage 'build_image.bicep' = {
  name: 'build_image'
  params: {
    acrName: acr.name
    doBuildContainerAppImage: doBuildContainerAppImage
    location: location
    imageWithTag: imageWithTag
    githubApiRepositoryUrl: githubApiRepositoryUrl
    githubApiRepositoryBranch: githubApiRepositoryBranch
  }
}

// Chapter 001: VNET and Subnets
resource vnet 'Microsoft.Network/virtualNetworks@2023-05-01' = {
  name: vnetName
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: [
        vnetAddressSpace
      ]
    }
    subnets: [
      {
        name: containerAppSubnetName
        properties: {
          addressPrefix: appEnvSubnetCidr
          networkSecurityGroup: (enableIpWhitelist) ? {
            id: nsgAllowIpWhitelist.id
          } : null
        }
      }
    ]
  }
  resource containerappSubnet 'subnets' existing = {
    name: containerAppSubnetName
  }
}

resource synapseWorkspace 'Microsoft.Synapse/workspaces@2021-06-01' = if (deploySynapse) {
  name: synapseWorkspaceName
  location: location
  identity: {
    type: 'SystemAssigned,UserAssigned'
    userAssignedIdentities: {
      '${uai.id}': {}
    }
  }
  properties: {
    defaultDataLakeStorage: {
      accountUrl: 'https://${storageAccountName}.dfs.${environment().suffixes.storage}'
      filesystem: blobContainerName
      resourceId: sa.id
      createManagedPrivateEndpoint: false
    }
    azureADOnlyAuthentication: true
  }

  resource synapseAllowAll 'firewallRules' = if (deploySynapse) {
    name: 'allowAll'
    properties: {
      startIpAddress: '0.0.0.0'
      endIpAddress: '255.255.255.255'
    }
  }
}

resource synapseAdx 'Microsoft.Synapse/workspaces/kustoPools@2021-06-01-preview' = {
  parent: synapseWorkspace
  name: adxPoolName
  location: location
  sku: {
    capacity: 2
    name: 'Compute optimized'
    size: 'Extra small'
  }
  properties: {
    enableStreamingIngest: true
    enablePurge: true
    workspaceUID: synapseWorkspace.properties.workspaceUID
    optimizedAutoscale: {
      version: 1
      isEnabled: true
      minimum: 2
      maximum: 3
    }
  }
  resource database 'databases' = {
    kind: 'ReadWrite'
    location: location
    name: adxDatabaseName
    properties: {
      hotCachePeriod: 'P31D'
    }
  }
}

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

resource uaiRbac 'Microsoft.Authorization/roleAssignments@2022-04-01' = if (useManagedIdentity) {
  name: guid(resourceGroup().id, uai.id, acrPullRole)
  scope: acr
  properties: {
    roleDefinitionId: acrPullRole
    principalId: uai.properties.principalId
    principalType: 'ServicePrincipal'
  }
}

resource uaiRbacStorage 'Microsoft.Authorization/roleAssignments@2022-04-01' = if (useManagedIdentity) {
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

resource blobContainer 'Microsoft.Storage/storageAccounts/blobServices/containers@2023-01-01' = {
  name: blobContainerName
  parent: blobServices
  properties: {}
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
    vnetConfiguration: {
      infrastructureSubnetId: vnet::containerappSubnet.id
    }
  }
}

resource containerApp 'Microsoft.App/containerApps@2023-05-01' = if (deployApps) {
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
          value: acr.listCredentials().passwords[0].value
        }
        {
          name: 'storageaccountkey'
          value: sa.listKeys().keys[0].value
        }
        {
          name: 'redispassword'
          value: redisCache.listKeys().primaryKey
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
          image: '${acr.properties.loginServer}/${buildContainerImage.outputs.imageWithTag}'
          env: [
            {
              name: 'OMIT_STARTUP_CHECK'
              value: 'true'
            }
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
              name: 'REQUIRE_API_KEY'
              value: apiKeyToUse
            }
            {
              name: 'REDIS_PASSWORD'
              secretRef: 'redispassword'
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

resource nsgAllowIpWhitelist 'Microsoft.Network/networkSecurityGroups@2023-05-01' = if (enableIpWhitelist) {
  name: 'nsg-whitelsit-${projectName}-${salt}'
  location: location
  properties: {
    securityRules: [
      {
        name: 'AllowMyIpAddressHTTPSInboundToAnything'
        type: 'Microsoft.Network/networkSecurityGroups/securityRules'
        properties: {
          protocol: 'TCP'
          sourcePortRange: '*'
          destinationPortRange: '443'
          destinationAddressPrefix: '*'
          access: 'Allow'
          priority: 2711
          direction: 'Inbound'
          sourceAddressPrefixes: split(ipWhitelist, ',')
        }
      }
      {
        name: 'AllowTagHTTPSInbound'
        type: 'Microsoft.Network/networkSecurityGroups/securityRules'
        properties: {
          protocol: 'TCP'
          sourcePortRange: '*'
          destinationPortRange: '30000-32676'
          sourceAddressPrefix: 'AzureLoadBalancer'
          destinationAddressPrefix: '*'
          access: 'Allow'
          priority: 2810
          direction: 'Inbound'
        }
      }
    ]
  }
}

var keyAppendix = length(apiKeyToUse) > 0 ? '?key=${apiKeyToUse}' : ''
output containerAppFQDN string = (deployApps) ? '${containerApp.properties.configuration.ingress.fqdn}${keyAppendix}' : 'https://<containerAppFQDN>'
output containerAppStaticIP string = containerAppEnv.properties.staticIp
output apiKey string = apiKeyToUse
