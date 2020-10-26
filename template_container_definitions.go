package dulumi

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

func ContainerDefinitionTemplate(
	appImage string,
	appPort string,
	awsLogsGroup string,
	appEnvs map[string]string,
	secretArn *string,
	appSecrets map[string]string,
	appEnableLogrouter bool,
) string {

	type ContainerParameter struct {
		AppImage, AppPort, AwsLogsGroup, AppEnvs, AppSecrets string
		AppEnableLogrouter                                   bool
	}

	var envs []string
	for k, v := range appEnvs {
		envs = append(envs, env(k, v))
	}

	var secrets []string
	for k := range appSecrets {
		secrets = append(secrets, secret(secretArn, k))
	}

	param := ContainerParameter{
		appImage,
		appPort,
		awsLogsGroup,
		strings.Join(envs, ","),
		strings.Join(secrets, ","),
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

func env(key string, value string) string {
	return fmt.Sprintf(`{
	"name": "%v",
	"value": "%v"
	}`, key, value)
}

func secret(secretArn *string, secret string) string {
	return fmt.Sprintf(`{
	"valueFrom": "%v:%v::",
	"name": "%v"
	}`, *secretArn, secret, secret)
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
    "environment": [],
    "mountPoints": [],
    "volumesFrom": [],
    "image": "784015586554.dkr.ecr.ap-northeast-1.amazonaws.com/mybridge-aws-fluent-bit",
    "firelensConfiguration": {
      "type": "fluentbit",
      "options": {
        "config-file-type": "file",
        "config-file-value": "/fluent-bit/etc/mybridge-fluent-bit.conf"
      }
    },
    "user": "0",
    "name": "log-router"
  }
{{end}}
]`
