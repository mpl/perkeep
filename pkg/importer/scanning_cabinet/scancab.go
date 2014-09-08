/*
Copyright 2014 The Camlistore Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"code.google.com/p/goauth2/oauth"

	"github.com/golang/oauth2"
	"github.com/golang/oauth2/google"
	gcdatastore "google.golang.org/cloud/datastore"
	gcstorage "google.golang.org/cloud/storage"
	gcstorage2 "github.com/GoogleCloudPlatform/gcloud-golang/storage"

	oauth1 "camlistore.org/third_party/github.com/garyburd/go-oauth/oauth"
)

/*
To get blobs, already tried:
1) gcs with google.golang.org/cloud/storage -> getting a 403. Plus that wouldn't work anyway
as the blobs don't seem to be in the bucket when I ls with gsutil.
2) oauth2 with code.google.com/p/goauth2/oauth + GET on /resource -> getting redirected.
3) oauth2 with github.com/golang/oauth2 + GET on /resource -> same thing.
4) oauth with github.com/garyburd/go-oauth/oauth -> getting a 400, but maybe I half-assed it.
5) went back to github.com/golang/oauth2, and added X-AppEngine-User-Email header -> not better.
6) go doc hinted at the problem: there's still a login: required in app.yaml, that oauth does not override. need to test and confirm (that we're ok without it).
*/

var (
	projectId = "scancabcamli"
	serviceAccount = "886924983567-uiln6pus9iuumdq3i0vav0ntveodas0r@developer.gserviceaccount.com"
	myEmail = "mathieu.lonjaret@gmail.com"
	ds *gcdatastore.Dataset
	cl *http.Client
	clientId = "886924983567-hnd1dertfvi2g0lpjs72aae8hi35k364.apps.googleusercontent.com"
	clientSecret = "XYta4alhMIjTHK5mreBk-fXU"
	oa1tempCreds *oauth1.Credentials
	oa1Creds *oauth1.Credentials
	oa1CallbackURL = "http://home.granivo.re:8009/callback"
	host = ":8080"
	gotCreds chan struct{}
)

// UserInfo represents the metadata associated with the Google User
// currently logged-in to the app
type UserInfo struct {
	// User stores the email address of the currently logged-in user
	// this is used as the primary key
	User string

	// MediaObjects is a count of the MediaObjects currently associated with this user
	MediaObjects int64

	// UploadPassword is a plain-text string that protects the scan upload API
	UploadPassword string
}

// MediaObject represents the metadata associated with each individual uploaded scan
type MediaObject struct {
	// Owner is the key of the UserInfo of the user that uploaded the file
	Owner *gcdatastore.Key 

	// IntID is the entity ID of the key associated with this MediaObject struct
	// Not stored in datastore but filled on each get()
	IntID int64 `datastore:"-"`

	// Blob is the key of blobstore entry with this uploaded file
	Blob string

	// Creation the time when this struct was originally created
	Creation time.Time 

	// ContentType is the MIME-type of the uploaded file.
	// As the mime/multipart package does not detect Content-Type
	// before sending the file in the command line client, this is
	// detected in the webapp and so this field may differ from the
	// content-type for the associated blob in the blobstore
	ContentType string 

	// Filename is the name of the file when it was uploaded
	Filename string 

	// Size in bytes of the uploaded file
	Size int64

	// Document is the key of the associated Document struct.
	// A Document has many MediaObjects. When newly uploaded,
	// a MediaObject is not associated with a Document.
	Document *gcdatastore.Key

	// LacksDocument is false when this MediaObject is associated with a Document.
	// When newly uploaded, a MediaObject is not associated with a Document.
	LacksDocument bool
}

// Document is a structure that groups scans into a logical unit.
// A letter (Stored as a document) could have several pages
// (each is a MediaObject), for example.
type Document struct {
	// Owner is the key of the UserInfo of the user that created the Document
//	Owner *gcdatastore.Key

	// Pages are the keys of each Media Object that contitute this Document
//	Pages []*gcdatastore.Key

	// IntID is the entity ID of the key associated with this Document struct
	// Not stored in datastore but filled on each get()
	IntID int64 `datastore:"-"`

	// DocDate is the user-nominated date associated with this document. It can
	// store any date the user likes but is intended to be when the document was
	// received, or, perhaps, written or sent
	DocDate time.Time 

	// NoDate is false when DocDate has been set by the user
	NoDate bool

	// Creation is the date the Document struct was created
	Creation time.Time

	// Title is the user-nominated title of the document
	Title string

	// Description is the user-nominated description of the document
	Description string

	// Tags is the slice of zero or more tags associated with the document by the user
	Tags string

	// LowercaseTags is the content of Tags but stored lowercase as a
	// canonical version so searches on tags can be case-insensitive
	LowercaseTags string

	// NoTags is true when Tags is empty
	NoTags bool

	// PhysicalLocation is the user-nominated description of the location
	// of the physical document of which the MediaObjects associated with this
	// Document are scans 
	PhysicalLocation string

	// DueDate is the user-nominated date that the document is "due". The
	// meaning of what "due" means in relation to each particular document
	// is up to the user
	DueDate time.Time
}

const scansRequestLimit = 5

func getScans() ([]*MediaObject, error) {
	var scans []*MediaObject
	query := ds.NewQuery("MediaObject")
	query = query.Limit(scansRequestLimit)
	for {
		sc := make([]*MediaObject, scansRequestLimit)
		keys, next, err := ds.RunQuery(query, sc)
		if err != nil {
			return nil, err
		}
		scans = append(scans, sc...)
		// TODO(mpl): get the MediaObject IntId from the key
		for _,v := range keys {
			fmt.Printf("key: %v, ", v)
		}
		if next == nil {
			break
		}
		query = next
	}
	return scans, nil
}

func getScannedFile(key string) error {
//	"https://scancabcamli.appspot.com/resource/5066549580791808/glenda.png"
//	resp, err := cl.Get("https://scancabcamli.appspot.com/resource/"+key+"/glenda.png")
	req, err := http.NewRequest("GET", "https://scancabcamli.appspot.com/resource/"+key+"/glenda.png", nil)
	if err != nil {
		return err
	}
	req.Header.Add("X-AppEngine-User-Email", "mathieu.lonjaret@gmail.com")
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Status %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return ioutil.WriteFile("/home/mpl/glenda.png", body, 0700)
}

var ()

func transportFromAPIKey() (*oauth.Transport, error) {
        var config = &oauth.Config{
//				Scope: "https://www.googleapis.com/auth/userinfo.email",
				Scope: "https://www.googleapis.com/auth/appengine",
                ClientId:     clientId,
                ClientSecret: clientSecret,
                AuthURL:      "https://accounts.google.com/o/oauth2/auth",
                TokenURL:     "https://accounts.google.com/o/oauth2/token",
                RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
                TokenCache:   oauth.CacheFile("/home/mpl/tokencache.json"),
        }
        // Set up a Transport using the config.
        transport := &oauth.Transport{Config: config}

        // Try to pull the token from the cache; if this fails, we need to get one.
        token, err := config.TokenCache.Token()
        if err != nil {
                // Get an authorization code from the data provider.
                // ("Please ask the user if I can access this resource.")
                fmt.Printf("Visit this URL to get a code: %v\n", config.AuthCodeURL(""))
                input := bufio.NewReader(os.Stdin)
                line, _, err := input.ReadLine()
                if err != nil {
                        log.Fatalf("Failed to read line: %v", err)
                }
                code := strings.TrimSpace(string(line))

                // Exchange the authorization code for an access token.
                // ("Here's the code you gave the user, now give me a token!")
                token, err = transport.Exchange(code)
                if err != nil {
                        log.Fatalf("Exchange: %v", err)
                }
        }
        // Make the actual request using the cached token to authenticate.
        // ("Here's the token, let me in!")
        transport.Token = token
        return transport, nil
}

func transportFromAPIKey2() (*oauth2.Transport, error) {
	conf, err := oauth2.NewConfig(&oauth2.Options{
				Scopes: []string{"https://www.googleapis.com/auth/appengine.admin",
					"https://www.googleapis.com/auth/userinfo.email"},
                ClientID:     clientId,
                ClientSecret: clientSecret,
                RedirectURL:  "urn:ietf:wg:oauth:2.0:oob",
		},
		"https://accounts.google.com/o/oauth2/auth",
		"https://accounts.google.com/o/oauth2/token")
	if err != nil {
		return nil, err
	}

	// Redirect user to consent page to ask for permission
	// for the scopes specified above.
	url := conf.AuthCodeURL("state", "online", "auto")
//	url := conf.AuthCodeURL("state", "offline", "auto")
	fmt.Printf("Visit the URL for the auth dialog: %v\n", url)

	input := bufio.NewReader(os.Stdin)
	line, _, err := input.ReadLine()
	if err != nil {
		log.Fatalf("Failed to read line: %v", err)
	}
	authorizationCode := strings.TrimSpace(string(line))

	return conf.NewTransportWithCode(authorizationCode)
}

func oauth1Client() *oauth1.Client {
	return &oauth1.Client{
		TemporaryCredentialRequestURI: "https://scancabcamli.appspot.com/_ah/OAuthGetRequestToken",
		ResourceOwnerAuthorizationURI: "https://scancabcamli.appspot.com/_ah/OAuthAuthorizeToken",
		TokenRequestURI:               "https://scancabcamli.appspot.com/_ah/OAuthGetAccessToken",
		Credentials: oauth1.Credentials{
			Token:  clientId,
			Secret: clientSecret,
		},
	}
}

func tempCreds() {
	oauthClient := oauth1Client()
	tempCred, err := oauthClient.RequestTemporaryCredentials(&http.Client{}, oa1CallbackURL, nil)
	if err != nil {
		log.Fatalf("Error getting temp cred: %v", err)
	}
	oa1tempCreds = tempCred
}

func serveOAuthCallback(w http.ResponseWriter, r *http.Request) {
	oauthClient := oauth1Client()
	if oa1tempCreds.Token != r.FormValue("oauth_token") {
		log.Fatalf("unexpected oauth_token: got %v, want %v", r.FormValue("oauth_token"), oa1tempCreds.Token)
	}
	tokenCred, _, err := oauthClient.RequestToken(
		&http.Client{},
		&oauth1.Credentials{
			Token:  oa1tempCreds.Token,
			Secret: oa1tempCreds.Secret,
		},
		r.FormValue("oauth_verifier"),
	)
	if err != nil {
		log.Fatalf("Error getting final token: %v", err)
	}
	oa1Creds = tokenCred
	fmt.Fprintf(w, "whatever")
	gotCreds <- struct{}{}
}

func doOAuth1() {
	http.HandleFunc("/callback", serveOAuthCallback)
	go func() {
		log.Fatal(http.ListenAndServe(host, nil))
	}()
	gotCreds = make(chan struct{})
	tempCreds()
	<- gotCreds
	oauthClient := oauth1Client()
	resp, err := oauthClient.Get(&http.Client{},
		oa1Creds,
		"https://scancabcamli.appspot.com/resource/5066549580791808/glenda.png",
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Get request on failed with: %s", resp.Status)
	}		
	if resp.StatusCode != 200 {
		log.Fatalf("Status %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile("/home/mpl/glenda.png", body, 0700)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func main() {
	pemKeyBytes, err := ioutil.ReadFile("/home/mpl/scancabcamli-496f5f6eb01b.pem")
	if err != nil {
		log.Fatal(err)
	}
	conf, err := google.NewServiceAccountConfig(&oauth2.JWTOptions{
		Email:      serviceAccount,
		PrivateKey: pemKeyBytes,
		Scopes:     []string{
			gcstorage2.ScopeFullControl,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	bkt := gcstorage2.New(conf.NewTransport()).Bucket(projectId)
	_, err = bkt.Stat("glenda.png")
	if err != nil {
		log.Fatalf("stat: %v", err)
	}
	return

	tr, err := transportFromAPIKey2()
	if err != nil {
		log.Fatal(err)
	}
	cl = &http.Client{Transport: tr}
	scanBlobKey := "5066549580791808"
	if err := getScannedFile(scanBlobKey); err != nil {
		log.Fatal(err)
	}
	return

	cl = &http.Client{Transport: conf.NewTransport()}
//	scanBlobKey := "5066549580791808"
	if err := getScannedFile(scanBlobKey); err != nil {
		log.Fatal(err)
	}

	println("STORAGE")
	bk, err := gcstorage.NewBucket(projectId, serviceAccount, pemKeyBytes)
	if err != nil {
		log.Fatal(err)
	}
	_, err = bk.Stat("datastore_backup_datastore_backup_2014_09_10_UserInfo-1576947681876AC4320CD-output-0-attempt-1")
	if err != nil {
		log.Fatalf("stat: %v", err)
	}

	objs, _, err := bk.List(nil)
	if err != nil {
		log.Fatalf("list: %v", err)
	}
	for _,v := range objs {
		fmt.Printf("%T\n", v)
	}
	return

	ds, err = gcdatastore.NewDataset(projectId, serviceAccount, pemKeyBytes)
	if err != nil {
		log.Fatal(err)
	}
	scans, err := getScans()
	if err != nil {
		log.Fatal(err)
	}
	for _,v := range scans {
		fmt.Printf("%v\n", v)
		if v != nil && v.Owner != nil {
			userInfo := &UserInfo{}
			if err := ds.Get(v.Owner, userInfo); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Owner: %v\n", userInfo)
		}
		if v != nil && v.Document != nil {
			document := &Document{}
			if err := ds.Get(v.Document, document); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Document: %v\n", document)
		}
	}
}

/*

//	"code.google.com/p/goauth2/oauth/jwt"
//	"code.google.com/p/google-api-go-client/datastore/v1beta2"

func withAutoGoogAPI() {
	svc, err := datastore.New(transport.Client())
	if err != nil {
		log.Fatalf("New service: %v", err)
	}
	dsvc := datastore.NewDatasetsService(svc)

	rqr := datastore.RunQueryRequest{
		Query: &datastore.Query{
			Kinds: []*datastore.KindExpression{
				{
					Name: "UserInfo",
				},
			},
			Filter: &datastore.Filter{
				PropertyFilter: &datastore.PropertyFilter{
					Operator: "equal",
					Property: &datastore.PropertyReference{"User"},
					Value: &datastore.Value{
						StringValue: email,
					},
				},
			},
		},
	}

	// RunQuery: Query for entities.
	call := dsvc.RunQuery("scancabcamli", &rqr)

	lookup := datastore.LookupRequest{
		Keys: []*datastore.Key{
			{
			Path: []*datastore.KeyPathElement{
				{
					Kind: "Document",
					Id: 5144752345317376,
				},
			},
			},
		},
	}
	call := dsvc.Lookup("scancabcamli", &lookup)
	_, err = call.Do()
	if err != nil {
		log.Fatalf("query: %v", err)
	}
}

func withJWT() {
	// Craft the ClaimSet and JWT token.
	iss := "886924983567-uiln6pus9iuumdq3i0vav0ntveodas0r@developer.gserviceaccount.com"
	//scope := "https://www.googleapis.com/auth/devstorage.read_only"
	scope := datastore.DatastoreScope
	pemKeyBytes, err := ioutil.ReadFile("/home/mpl/scancabcamli-496f5f6eb01b.pem")
	if err != nil {
		log.Fatal(err)
	}
//	tr, err := jwt.NewTransport(jwt.NewToken(iss, scope, pemKeyBytes))
	_, err = jwt.NewTransport(jwt.NewToken(iss, scope, pemKeyBytes))
	if err != nil {
		log.Fatal(err)
	}
}

*/
