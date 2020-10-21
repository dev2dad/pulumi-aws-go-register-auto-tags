package dulumi

import (
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/cloudfront"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/route53"
	"github.com/pulumi/pulumi-aws/sdk/v3/go/aws/s3"
	"log"

	plm "github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

type S3StaticWeb struct {
	plm.ResourceState

	BucketName     plm.IDOutput `pulumi:"bucketName"`
	DistributionId plm.IDOutput `pulumi:"id"`
}

func NewS3StaticWeb(ctx *plm.Context,
	host string,
	domain string,
	envService string,
	domainSslCertArn string,
	opts ...plm.ResourceOption) (*S3StaticWeb, error) {

	var dsw S3StaticWeb
	err := ctx.RegisterComponentResource("drama:web:s3-static", "drama-s3-static-web", &dsw, opts...)
	if err != nil {
		return nil, err
	}

	bucket, err := s3.NewBucket(ctx, "bucket", &s3.BucketArgs{
		Acl:    plm.String("private"),
		Bucket: plm.String(host),
		Versioning: s3.BucketVersioningArgs{
			Enabled: plm.Bool(true),
		},
		Website: s3.BucketWebsiteArgs{
			RedirectAllRequestsTo: plm.String(fmt.Sprintf("https://%v", plm.String(host))),
		},
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	zone, err := route53.LookupZone(ctx, &route53.LookupZoneArgs{
		Name: &domain,
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	originAccessIdentity, err := cloudfront.NewOriginAccessIdentity(ctx, "originAccessIdentity", &cloudfront.OriginAccessIdentityArgs{
		Comment: plm.String(host),
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	distribution, err := cloudfront.NewDistribution(ctx, "distribution", &cloudfront.DistributionArgs{
		Origins: cloudfront.DistributionOriginArray{
			&cloudfront.DistributionOriginArgs{
				DomainName: bucket.BucketRegionalDomainName,
				OriginId:   plm.String(fmt.Sprintf("S3-%v", envService)),
				S3OriginConfig: &cloudfront.DistributionOriginS3OriginConfigArgs{
					OriginAccessIdentity: originAccessIdentity.CloudfrontAccessIdentityPath,
				},
			},
		},
		Restrictions: cloudfront.DistributionRestrictionsArgs{
			GeoRestriction: cloudfront.DistributionRestrictionsGeoRestrictionArgs{
				RestrictionType: plm.String("none"),
			},
		},
		Enabled:           plm.Bool(true),
		IsIpv6Enabled:     plm.Bool(true),
		Comment:           plm.String(host),
		DefaultRootObject: plm.String("index.html"),
		DefaultCacheBehavior: cloudfront.DistributionDefaultCacheBehaviorArgs{
			AllowedMethods:       plm.StringArray{plm.String("GET"), plm.String("HEAD")},
			CachedMethods:        plm.StringArray{plm.String("GET"), plm.String("HEAD")},
			DefaultTtl:           plm.Int(86400),
			MinTtl:               plm.Int(0),
			MaxTtl:               plm.Int(31536000),
			TargetOriginId:       plm.String(fmt.Sprintf("S3-%v", envService)),
			ViewerProtocolPolicy: plm.String("redirect-to-https"),
			ForwardedValues: cloudfront.DistributionDefaultCacheBehaviorForwardedValuesArgs{
				QueryString: plm.Bool(false),
				Cookies: cloudfront.DistributionDefaultCacheBehaviorForwardedValuesCookiesArgs{
					Forward: plm.String("none"),
				},
			},
		},
		ViewerCertificate: cloudfront.DistributionViewerCertificateArgs{
			AcmCertificateArn: plm.String(domainSslCertArn),
		},
	}, plm.Parent(&dsw),
		plm.IgnoreChanges([]string{"tags"}))
	if err != nil {
		return nil, err
	}

	_, err = route53.NewRecord(ctx, "record", &route53.RecordArgs{
		Name:   plm.String(host),
		Type:   plm.String("A"),
		ZoneId: plm.String(zone.ZoneId),
		Aliases: route53.RecordAliasArray{
			&route53.RecordAliasArgs{
				Name:                 distribution.DomainName,
				ZoneId:               distribution.HostedZoneId,
				EvaluateTargetHealth: plm.Bool(true),
			},
		},
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	allow := "Allow"
	sid := "2"
	policy, err := iam.GetPolicyDocument(ctx, &iam.GetPolicyDocumentArgs{
		Statements: []iam.GetPolicyDocumentStatement{
			{
				Sid: &sid,
				Effect: &allow,
				Actions: []string{
					"s3:GetObject",
				},
				Resources: []string{
					fmt.Sprintf("%v/*", bucket.Arn),
				},
				Principals: []iam.GetPolicyDocumentStatementPrincipal{
					{
						Type: "AWS",
						Identifiers: []string{
							fmt.Sprintf("%v", originAccessIdentity.IamArn),
						},
					},
				},
			},
		},
	}, plm.Parent(&dsw))
	if err != nil {
		return nil, err
	}

	log.Print(fmt.Sprintf("%v", policy.Json))

	if _, err := s3.NewBucketPolicy(ctx, "bucketPolicy", &s3.BucketPolicyArgs{
		Bucket: bucket.Bucket,
		Policy: plm.String(fmt.Sprintf("%v", policy.Json)),
	}, plm.Parent(&dsw)); err != nil {
		return nil, err
	}

	dsw.BucketName = bucket.ID()
	if err = ctx.RegisterResourceOutputs(&dsw, plm.Map{
		"bucketName":     bucket.Bucket,
		"distributionId": distribution.ID(),
	}); err != nil {
		return nil, err
	}

	return &dsw, nil
}
