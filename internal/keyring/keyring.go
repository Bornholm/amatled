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

// DeleteProfile supprime toutes les entrées keyring associées à un profil.
func DeleteProfile(profileName string) error {
	_ = DeleteProfileAPIKey(profileName)
	return nil
}

func SetRenderPresetPassword(presetName, password string) error {
	return goKeyring.Set(service, "render-preset/"+presetName+"/password", password)
}

func GetRenderPresetPassword(presetName string) (string, error) {
	return goKeyring.Get(service, "render-preset/"+presetName+"/password")
}

func DeleteRenderPresetPassword(presetName string) error {
	return goKeyring.Delete(service, "render-preset/"+presetName+"/password")
}

// DeleteRenderPreset supprime toutes les entrées keyring associées à un préset de rendu.
func DeleteRenderPreset(presetName string) error {
	_ = DeleteRenderPresetPassword(presetName)
	return nil
}
