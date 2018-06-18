package registry

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
)

var (
	registryURI string
	protocol    = "https"
)

// Image contains parsed image info
type Image struct {
	Registry  string
	ImageName string
	Tag       string
	Token     string
}

// SetDefaultRegistry sets registry URL to be used when not provided
func SetDefaultRegistry(uri string) {
	registryURI = uri
}

// InsecureRegistry sets registry protocol to http if true.
// We won't support self signed ones as getting proper cert
// in 2017 is not a big deal (letsencrypt.org)
func InsecureRegistry(insecure bool) {
	if insecure {
		protocol = "http"
	}
}

// ParseImageName try to parse image name in form [registry.domain][/image/]name[:tag]
func ParseImageName(name string) (*Image, error) {
	img := new(Image)
	var err error
	repo := strings.SplitN(name, "/", 2)
	if len(repo) == 1 {
		//no custom registry, using default one

		img.Registry, err = getDefaultRegistry()
		if err != nil {
			return &Image{}, err
		}
		var imgName string
		// get proper tag
		imgName, img.Tag = getTag(repo[0])
		// check if we need to add "library/"
		img.ImageName = checkIfLibrary(imgName)
	} else {
		// check for . in repo[0]. If present we have custom registry.
		if strings.Contains(repo[0], ".") {
			img.Registry = repo[0]
			img.ImageName, img.Tag = getTag(repo[1])
		} else {
			// no custom registry
			img.Registry, err = getDefaultRegistry()
			if err != nil {
				return &Image{}, err
			}
			// we need to add 1st part of image name here
			img.ImageName, img.Tag = getTag(repo[0] + "/" + repo[1])
		}
	}

	return img, nil
}

// checks if tag is present. if not latest is returned.
// returns image name string and tag
func getTag(image string) (string, string) {
	imgName := strings.SplitN(image, ":", 2)
	if len(imgName) == 1 {
		//no tag defined, using default
		return imgName[0], "latest"
	}
	return imgName[0], imgName[1]
}

// check if image name needs "library/" prefix
// and returns proper image name
func checkIfLibrary(image string) string {
	//if image is from official docker registry and doesn't have username
	//"library" should be added. Simplest test is checking if 2nd / is present
	if strings.Contains(image, "/") {
		// / is present nothing to add
		return image
	}
	//we need to add "library"
	return "library/" + image
}

func getDefaultRegistry() (string, error) {
	if registryURI == "" {
		return "", errors.New("No default registry configured")
	}
	return registryURI, nil
}

// In order to download something from docker hub or other compatible registry we need to have token. Even for anonymous stuff.
func (i *Image) getAuthToken() error {
	//first we need to check response to GET /v2/ if we will get unauthorized then we will need to obtain token
	resp, err := http.Get(protocol + "://" + i.Registry + "/v2/")
	if err != nil {
		return err
	}
	var realm, service string
	if resp.StatusCode == http.StatusUnauthorized {
		// we need token for this repo. WWW-Authenticate header will tell us where to get it
		re := regexp.MustCompile(`Bearer realm="(?P<realm>.*)",service="(?P<service>.*)"`)
		parsed := re.FindStringSubmatch(resp.Header.Get("WWW-Authenticate"))
		realm, service = parsed[1], parsed[2]
	} else {
		//no token needed
		return nil
	}
	resp, err = http.Get(realm + "?service=" + service + "&scope=repository:" + i.ImageName + ":pull")

	if err != nil {
		return err
	}
	var authResponse interface{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&authResponse)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("Auth status code: " + resp.Status)
	}
	/*
	 Line below might look bit cryptic to newcomers but it's quite simple
	 we decoded JSON response into interface{} datatype, but encoding/json package
	 put map[string]interface{} there. We know that we want "token" field from json
	 and it is a string. So we type asserted first interface{} to map[string]interface{}
	 and then interface{} from map to string. We could avoid all this by creating
	 proper struct for response json and parse json directly to this struct, but I'm too lazy ;)
	 drawback: it will crash when asserted types doesn't match,
	 but as all this is just for blog it's fine. Remember all type assertions presented in this
	 code were made by professionals don't try this at home.
	*/
	i.Token = authResponse.(map[string]interface{})["token"].(string)

	return nil
}
