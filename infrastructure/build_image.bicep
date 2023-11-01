param acrName string
param doBuildContainerAppImage bool
param location string
param imageWithTag string
param githubApiRepositoryUrl string
param githubApiRepositoryBranch string

module containerAppImageBuild 'br/public:deployment-scripts/build-acr:2.0.2' = if(doBuildContainerAppImage) {
  name: 'build-container-app-image'
  params: {
    AcrName: acrName
    location: location
    gitRepositoryUrl:  githubApiRepositoryUrl
    buildWorkingDirectory: 'src'
    imageName: split(imageWithTag, ':')[0]
    imageTag: split(imageWithTag, ':')[1]
    gitBranch: githubApiRepositoryBranch
  }
}

output imageWithTag string = imageWithTag
