package keyring

import goKeyring "github.com/zalando/go-keyring"

const service = "amatled"

func SetProfileAPIKey(profileName, apiKey string) error {
	return goKeyring.Set(service, "profile/"+profileName+"/llm-apikey", apiKey)
}

func GetProfileAPIKey(profileName string) (string, error) {
	return goKeyring.Get(service, "profile/"+profileName+"/llm-apikey")
}

func DeleteProfileAPIKey(profileName string) error {
	return goKeyring.Delete(service, "profile/"+profileName+"/llm-apikey")
}

func SetProfileRenderPassword(profileName, password string) error {
	return goKeyring.Set(service, "profile/"+profileName+"/render-password", password)
}

func GetProfileRenderPassword(profileName string) (string, error) {
	return goKeyring.Get(service, "profile/"+profileName+"/render-password")
}

func DeleteProfileRenderPassword(profileName string) error {
	return goKeyring.Delete(service, "profile/"+profileName+"/render-password")
}

// DeleteProfile supprime toutes les entrées keyring associées à un profil.
func DeleteProfile(profileName string) error {
	_ = DeleteProfileAPIKey(profileName)
	_ = DeleteProfileRenderPassword(profileName)
	return nil
}
