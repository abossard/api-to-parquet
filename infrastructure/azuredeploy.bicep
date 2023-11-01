@description('The location to deploy to.')
param location string = resourceGroup().location

@description('List of IP addresses to whitelist. Separate with commas. Leave empty for public access.')
param ipWhitelist string = ''

module main 'main.bicep' = {
  name: 'main'
  params: {
    location: location
    ipWhitelist: ipWhitelist
    enableIpWhitelist: ipWhitelist != ''
  }
}

output endpoint string = main.outputs.containerAppFQDN
output staticIp string = main.outputs.containerAppStaticIP
output apiKey string = main.outputs.apiKey
