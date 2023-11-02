@description('The location to deploy to.')
param location string = resourceGroup().location

@description('List of IP addresses to whitelist. Separate with commas. Leave empty for public access.')
param ipWhitelist string = ''

@description('Skip building and pushing the container image. Use this if you have already built and pushed the image.')
param skipBuildAndPushContainerImage bool = false

module main 'main.bicep' = {
  name: 'main'
  params: {
    location: location
    ipWhitelist: ipWhitelist
    enableIpWhitelist: ipWhitelist != ''
    doBuildContainerAppImage: !skipBuildAndPushContainerImage
  }
}

output endpoint string = main.outputs.containerAppFQDN
output staticIp string = main.outputs.containerAppStaticIP
output apiKey string = main.outputs.apiKey
