package dulumi

const ECR_LIFECYCLE_POLICY = `
{
    "rules": [
        {
            "rulePriority": 1,
            "description": "Expire images more than 30",
            "selection": {
                "tagStatus": "any",
                "countType": "imageCountMoreThan",
                "countNumber": 30
            },
            "action": {
                "type": "expire"
            }
        }
    ]
}`
