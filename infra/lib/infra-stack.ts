import { Stack, Construct, StackProps } from "@aws-cdk/core"
import { Project, BuildSpec, LinuxBuildImage } from '@aws-cdk/aws-codebuild'
import { Bucket, BucketEncryption } from '@aws-cdk/aws-s3'
import { ManagedPolicy } from "@aws-cdk/aws-iam";

export class CodeBuilderStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);
    
    const artifactBucket = new Bucket(this, 'Artifacts', {
      encryption: BucketEncryption.KMS_MANAGED,
    });
    
    const sourceBucket = new Bucket(this, 'Sources', {
      encryption: BucketEncryption.KMS_MANAGED,
    });

    const buildProject = new Project(this, 'Build', {
      environment: {
        privileged: true,
        buildImage: LinuxBuildImage.STANDARD_2_0,
      },
      buildSpec: BuildSpec.fromObject({
        version: '0.2',
        phases: {
          install: {
            'runtime-versions': {
              docker: "18",
            }
          },
          build: {
            commands: [
              'make ci'
            ]
          }
        }
      })
    })

    artifactBucket.grantReadWrite(buildProject.role!);
    sourceBucket.grantRead(buildProject.role!);

    const deployProject = new Project(this, 'Deploy', {
      environment: {
        privileged: true, // required for the docker runtime!!
        buildImage: LinuxBuildImage.STANDARD_2_0,
      },
      // default build spec which provides the docker runtime
      buildSpec: BuildSpec.fromObject({
        version: '0.2',
        phases: {
          install: {
            'runtime-versions': {
              docker: "18",
            }
          },
          build: {
            commands: [
              'make ci'
            ]
          }
        }
      })
    })

    artifactBucket.grantReadWrite(deployProject.role!);
    sourceBucket.grantRead(deployProject.role!);

    deployProject.role!.addManagedPolicy(ManagedPolicy.fromAwsManagedPolicyName("AdministratorAccess"))

  }
}
