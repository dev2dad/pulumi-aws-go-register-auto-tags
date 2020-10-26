package dulumi

import "fmt"

func BuildSpecTemplate(ecrName string) string {
	return fmt.Sprintf(`
version: 0.2

phases:
  pre_build:
    commands:
      - aws --version
      - ECR=784015586554.dkr.ecr.ap-northeast-1.amazonaws.com
      - $(aws ecr get-login --no-include-email)
      - IMAGE_REPO_NAME=%v
      - IMAGE_TAG="$(echo $CODEBUILD_RESOLVED_SOURCE_VERSION)"
      - printf $IMAGE_TAG
      - IMAGE_URI="$ECR/$IMAGE_REPO_NAME:$IMAGE_TAG"
      - printf $IMAGE_URI
      - IMAGE_LATEST="$ECR/$IMAGE_REPO_NAME:latest"
  build:
    commands:
      - echo Building the Docker image...
      - docker build -t $IMAGE_REPO_NAME:latest .
      - docker tag $IMAGE_REPO_NAME:latest $IMAGE_URI
      - docker tag $IMAGE_REPO_NAME:latest $IMAGE_LATEST
  post_build:
    commands:
      - echo Pushing the Docker images...
      - docker push $IMAGE_URI
      - docker push $IMAGE_LATEST
      - echo Writing imagedefinitions.json...
      - printf '[{"name":"app","imageUri":"%%s"}]' $IMAGE_URI > imagedefinitions.json
      - cat imagedefinitions.json
artifacts:
  files:
    - imagedefinitions.json
`, ecrName)
}
