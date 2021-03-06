// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/multicloud/aliyun"
)

type SAliyunProviderFactory struct {
	cloudprovider.SPublicCloudBaseProviderFactor
}

func (self *SAliyunProviderFactory) GetId() string {
	return aliyun.CLOUD_PROVIDER_ALIYUN
}

func (self *SAliyunProviderFactory) GetName() string {
	return aliyun.CLOUD_PROVIDER_ALIYUN_CN
}

func (self *SAliyunProviderFactory) IsCloudeventRegional() bool {
	return true
}

func (self *SAliyunProviderFactory) IsSupportCloudIdService() bool {
	return true
}

func (self *SAliyunProviderFactory) IsSupportCreateCloudgroup() bool {
	return true
}

func (factory *SAliyunProviderFactory) IsSystemCloudpolicyUnified() bool {
	return false
}

func (self *SAliyunProviderFactory) GetSupportedDnsZoneTypes() []cloudprovider.TDnsZoneType {
	return []cloudprovider.TDnsZoneType{
		cloudprovider.PublicZone,
		cloudprovider.PrivateZone,
	}
}

func (self *SAliyunProviderFactory) GetSupportedDnsTypes() map[cloudprovider.TDnsZoneType][]cloudprovider.TDnsType {
	return map[cloudprovider.TDnsZoneType][]cloudprovider.TDnsType{
		cloudprovider.PublicZone: []cloudprovider.TDnsType{
			cloudprovider.DnsTypeA,
			cloudprovider.DnsTypeAAAA,
			cloudprovider.DnsTypeCAA,
			cloudprovider.DnsTypeCNAME,
			cloudprovider.DnsTypeMX,
			cloudprovider.DnsTypeNS,
			cloudprovider.DnsTypeSRV,
			cloudprovider.DnsTypeTXT,
			cloudprovider.DnsTypePTR,
			cloudprovider.DnsTypeFORWARD_URL,
			cloudprovider.DnsTypeREDIRECT_URL,
		},
		cloudprovider.PrivateZone: []cloudprovider.TDnsType{
			cloudprovider.DnsTypeA,
			cloudprovider.DnsTypeAAAA,
			cloudprovider.DnsTypeCNAME,
			cloudprovider.DnsTypeMX,
			cloudprovider.DnsTypeSRV,
			cloudprovider.DnsTypeTXT,
			cloudprovider.DnsTypePTR,
		},
	}
}

func (self *SAliyunProviderFactory) GetSupportedDnsPolicyTypes() map[cloudprovider.TDnsZoneType][]cloudprovider.TDnsPolicyType {
	return map[cloudprovider.TDnsZoneType][]cloudprovider.TDnsPolicyType{
		cloudprovider.PublicZone: []cloudprovider.TDnsPolicyType{
			cloudprovider.DnsPolicyTypeSimple,
			cloudprovider.DnsPolicyTypeByCarrier,
			cloudprovider.DnsPolicyTypeByGeoLocation,
			cloudprovider.DnsPolicyTypeBySearchEngine,
		},
		cloudprovider.PrivateZone: []cloudprovider.TDnsPolicyType{
			cloudprovider.DnsPolicyTypeSimple,
		},
	}
}

func (self *SAliyunProviderFactory) GetSupportedDnsValues() map[cloudprovider.TDnsPolicyType][]cloudprovider.TDnsPolicyValue {
	return map[cloudprovider.TDnsPolicyType][]cloudprovider.TDnsPolicyValue{
		cloudprovider.DnsPolicyTypeByCarrier: []cloudprovider.TDnsPolicyValue{
			cloudprovider.DnsPolicyValueUnicom,
			cloudprovider.DnsPolicyValueTelecom,
			cloudprovider.DnsPolicyValueChinaMobile,
			cloudprovider.DnsPolicyValueCernet,
		},
		cloudprovider.DnsPolicyTypeByGeoLocation: []cloudprovider.TDnsPolicyValue{
			cloudprovider.DnsPolicyValueOversea,
		},
		cloudprovider.DnsPolicyTypeBySearchEngine: []cloudprovider.TDnsPolicyValue{
			cloudprovider.DnsPolicyValueBaidu,
			cloudprovider.DnsPolicyValueGoogle,
			cloudprovider.DnsPolicyValueBing,
		},
	}
}

func (self *SAliyunProviderFactory) ValidateCreateCloudaccountData(ctx context.Context, userCred mcclient.TokenCredential, input cloudprovider.SCloudaccountCredential) (cloudprovider.SCloudaccount, error) {
	output := cloudprovider.SCloudaccount{}
	if len(input.AccessKeyId) == 0 {
		return output, errors.Wrap(httperrors.ErrMissingParameter, "access_key_id")
	}
	if len(input.AccessKeySecret) == 0 {
		return output, errors.Wrap(httperrors.ErrMissingParameter, "access_key_secret")
	}
	output.Account = input.AccessKeyId
	output.Secret = input.AccessKeySecret
	return output, nil
}

func (self *SAliyunProviderFactory) ValidateUpdateCloudaccountCredential(ctx context.Context, userCred mcclient.TokenCredential, input cloudprovider.SCloudaccountCredential, cloudaccount string) (cloudprovider.SCloudaccount, error) {
	output := cloudprovider.SCloudaccount{}
	if len(input.AccessKeyId) == 0 {
		return output, errors.Wrap(httperrors.ErrMissingParameter, "access_key_id")
	}
	if len(input.AccessKeySecret) == 0 {
		return output, errors.Wrap(httperrors.ErrMissingParameter, "access_key_secret")
	}
	output = cloudprovider.SCloudaccount{
		Account: input.AccessKeyId,
		Secret:  input.AccessKeySecret,
	}
	return output, nil
}

func (self *SAliyunProviderFactory) GetProvider(cfg cloudprovider.ProviderConfig) (cloudprovider.ICloudProvider, error) {
	client, err := aliyun.NewAliyunClient(
		aliyun.NewAliyunClientConfig(
			cfg.Account,
			cfg.Secret,
		).CloudproviderConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return &SAliyunProvider{
		SBaseProvider: cloudprovider.NewBaseProvider(self),
		client:        client,
	}, nil
}

func (self *SAliyunProviderFactory) GetClientRC(url, account, secret string) (map[string]string, error) {
	return map[string]string{
		"ALIYUN_ACCESS_KEY": account,
		"ALIYUN_SECRET":     secret,
		"ALIYUN_REGION":     aliyun.ALIYUN_DEFAULT_REGION,
	}, nil
}

func init() {
	factory := SAliyunProviderFactory{}
	cloudprovider.RegisterFactory(&factory)
}

type SAliyunProvider struct {
	cloudprovider.SBaseProvider
	client *aliyun.SAliyunClient
}

func (self *SAliyunProvider) GetSysInfo() (jsonutils.JSONObject, error) {
	regions := self.client.GetIRegions()
	info := jsonutils.NewDict()
	info.Add(jsonutils.NewInt(int64(len(regions))), "region_count")
	info.Add(jsonutils.NewString(aliyun.ALIYUN_API_VERSION), "api_version")
	return info, nil
}

func (self *SAliyunProvider) GetVersion() string {
	return aliyun.ALIYUN_API_VERSION
}

func (self *SAliyunProvider) GetSubAccounts() ([]cloudprovider.SSubAccount, error) {
	return self.client.GetSubAccounts()
}

func (self *SAliyunProvider) GetAccountId() string {
	return self.client.GetAccountId()
}

func (self *SAliyunProvider) GetIRegions() []cloudprovider.ICloudRegion {
	return self.client.GetIRegions()
}

func (self *SAliyunProvider) GetIRegionById(extId string) (cloudprovider.ICloudRegion, error) {
	return self.client.GetIRegionById(extId)
}

func (self *SAliyunProvider) GetBalance() (float64, string, error) {
	balance, err := self.client.QueryAccountBalance()
	if err != nil {
		return 0.0, api.CLOUD_PROVIDER_HEALTH_UNKNOWN, err
	}
	status := api.CLOUD_PROVIDER_HEALTH_NORMAL
	if balance.AvailableAmount <= 0 {
		status = api.CLOUD_PROVIDER_HEALTH_ARREARS
	} else if balance.AvailableAmount < 100 {
		status = api.CLOUD_PROVIDER_HEALTH_INSUFFICIENT
	}
	return balance.AvailableAmount, status, nil
}

func (self *SAliyunProvider) GetIProjects() ([]cloudprovider.ICloudProject, error) {
	return self.client.GetIProjects()
}

func (self *SAliyunProvider) CreateIProject(name string) (cloudprovider.ICloudProject, error) {
	return self.client.CreateIProject(name)
}

func (self *SAliyunProvider) GetStorageClasses(regionId string) []string {
	return []string{
		"Standard", "IA", "Archive",
	}
}

func (self *SAliyunProvider) GetBucketCannedAcls(regionId string) []string {
	return []string{
		string(cloudprovider.ACLPrivate),
		string(cloudprovider.ACLPublicRead),
		string(cloudprovider.ACLPublicReadWrite),
	}
}

func (self *SAliyunProvider) GetObjectCannedAcls(regionId string) []string {
	return []string{
		string(cloudprovider.ACLPrivate),
		string(cloudprovider.ACLPublicRead),
		string(cloudprovider.ACLPublicReadWrite),
	}
}

func (self *SAliyunProvider) GetCapabilities() []string {
	return self.client.GetCapabilities()
}

func (self *SAliyunProvider) GetIamLoginUrl() string {
	return self.client.GetIamLoginUrl()
}

func (self *SAliyunProvider) CreateIClouduser(conf *cloudprovider.SClouduserCreateConfig) (cloudprovider.IClouduser, error) {
	return self.client.CreateIClouduser(conf)
}

func (self *SAliyunProvider) GetICloudusers() ([]cloudprovider.IClouduser, error) {
	return self.client.GetICloudusers()
}

func (self *SAliyunProvider) GetICloudgroups() ([]cloudprovider.ICloudgroup, error) {
	return self.client.GetICloudgroups()
}

func (self *SAliyunProvider) GetICloudgroupByName(name string) (cloudprovider.ICloudgroup, error) {
	return self.client.GetICloudgroupByName(name)
}

func (self *SAliyunProvider) GetIClouduserByName(name string) (cloudprovider.IClouduser, error) {
	return self.client.GetIClouduserByName(name)
}

func (self *SAliyunProvider) CreateICloudgroup(name, desc string) (cloudprovider.ICloudgroup, error) {
	return self.client.CreateICloudgroup(name, desc)
}

func (self *SAliyunProvider) GetISystemCloudpolicies() ([]cloudprovider.ICloudpolicy, error) {
	return self.client.GetISystemCloudpolicies()
}

func (self *SAliyunProvider) GetICustomCloudpolicies() ([]cloudprovider.ICloudpolicy, error) {
	return self.client.GetICustomCloudpolicies()
}

func (self *SAliyunProvider) CreateICloudpolicy(opts *cloudprovider.SCloudpolicyCreateOptions) (cloudprovider.ICloudpolicy, error) {
	return self.client.CreateICloudpolicy(opts)
}

func (self *SAliyunProvider) GetSamlEntityId() string {
	return cloudprovider.SAML_ENTITY_ID_ALIYUN_ROLE
}

func (self *SAliyunProvider) GetSamlSpInitiatedLoginUrl(idpName string) string {
	return ""
}
