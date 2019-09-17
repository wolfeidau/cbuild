# cbuild

This is a command line tool which enables you to upload and build in the AWS using AWS CodeBuild. I wrote this tool to enable me to overcome the issue of uploading big bundles of code either in zip files or docker containers while on slow internet connections. This tool takes JUST your source code, builds an archive and pushes it to S3, then triggers a build either using a default `buildspec.yml` or the one in your projects root directory.

# Deployment

In the `infra` folder is an [AWS CDK](https://aws.amazon.com/cdk/) which deploys a couple of CodeBuild projects, one for building that has minimal access to AWS, and one for deploying which has admin privileges.

# Configuration

This tool expects some environment variables to be exported, in my case I use direnv to export them as I navigate to the project.

This is an example `.envrc` file, note the `xxxx` are placeholders for the identifiers which CDK will generate on deploy in your account.

```
export AWS_PROFILE=myprofile
export AWS_REGION=ap-southeast-2

export SOURCE_BUCKET="builderstack-dev-master-sourcesxxxx"
export ARTIFACT_BUCKET="builderstack-dev-master-artifactsxxxxx"
export BUILD_PROJECT_ARN="Buildxxxx"
export DEPLOY_PROJECT_ARN="Deployxxxx"
```

# Status

This project is still a work in progress and will be feature complete soon.

# License

This project is released under Apache 2.0 License.