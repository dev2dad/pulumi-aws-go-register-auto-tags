package dulumi

import (
	"bytes"
	"text/template"
)

func ContainerDefinitionTemplate(
	appImage string,
	appPort string,
	awsLogsGroup string,
	appEnvs map[string]string,
	secretArn *string,
	appSecrets map[string]string,
	logRouterEnvs map[string]string,
	appEnableLogrouter bool,
) string {

	type ContainerParameter struct {
		AppImage, AppPort, AwsLogsGroup, AppEnvs, AppSecrets, LogRouterEnvs string
		AppEnableLogrouter                                                  bool
	}

	envs := ContainerEnvs(appEnvs)

	secrets := ContainerSecrets(appSecrets, secretArn)

	logEnvs := ContainerEnvs(logRouterEnvs)

	param := ContainerParameter{
		appImage,
		appPort,
		awsLogsGroup,
		ContainerEnvJsonArray(envs),
		ContainerEnvJsonArray(secrets),
		ContainerEnvJsonArray(logEnvs),
		appEnableLogrouter,
	}

	t := template.Must(template.New("containerDefinition").Parse(definition))
	var buf bytes.Buffer
	err := t.Execute(&buf, param)
	if err != nil {
		panic("invalid containerDefinition")
	}
	return buf.String()
}

const definition = `[
  {
    "name": "app",
    "image": "{{.AppImage}}",
    "portMappings": [
      {
        "containerPort": {{.AppPort}},
        "hostPort": {{.AppPort}},
        "protocol": "tcp"
      }
    ],
    "environment": [
        {{.AppEnvs}}
    ],
    "ulimits": [{
      "name": "nofile",
      "softLimit": 65535,
      "hardLimit": 65535
    }],
    "secrets": [
        {{.AppSecrets}}
    ],
    "healthCheck": {
      "retries": 3,
      "command": [
        "CMD-SHELL",
        "echo hello"
      ],
      "timeout": 5,
      "interval": 30
    },
    "essential": true,
{{if .AppEnableLogrouter}}
    "logConfiguration": {
      "logDriver": "awsfirelens"
    }
},
{{else}}
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-group": "{{.AwsLogsGroup}}",
        "awslogs-region": "ap-northeast-1",
        "awslogs-stream-prefix": "app"
      }
    }
}
{{end}}
{{if .AppEnableLogrouter}}
  {
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-group": "{{.AwsLogsGroup}}",
        "awslogs-region": "ap-northeast-1",
        "awslogs-stream-prefix": "fluentbit"
      }
    },
    "portMappings": [
      {
        "hostPort": 24224,
        "protocol": "tcp",
        "containerPort": 24224
      }
    ],
    "cpu": 0,
    "ulimits": [{
      "name": "nofile",
      "softLimit": 65535,
      "hardLimit": 65535
    }],
    "environment": [
        {{.LogRouterEnvs}}
    ],
    "image": "784015586554.dkr.ecr.ap-northeast-1.amazonaws.com/drama-aws-fluent-bit:latest",
    "firelensConfiguration": {
      "type": "fluentbit",
      "options": {
        "config-file-type": "file",
        "config-file-value": "/fluent-bit/etc/drama-fluent-bit.conf"
      }
    },
    "user": "0",
    "name": "log-router"
  }
{{end}}
]`
