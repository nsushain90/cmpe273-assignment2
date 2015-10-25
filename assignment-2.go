package main

import (
	"encoding/json"
	"fmt"
	"github.com/jmoiron/jsonq"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type Location struct {
	Id         bson.ObjectId `json:"id" bson:"_id,omitempty"`
	Name       string        `json:"name"`
	Address    string        `json:"address"`
	City       string        `json:"city"`
	State      string        `json:"state"`
	Zip        string        `json:"zip"`
	Coordinate `json:"coordinate"`
}

type Coordinate struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func getCord(address, city, state, zip string) (lat, lng float64) {
	// request http api
	res, err := http.Get(strings.Replace("http://maps.google.com/maps/api/geocode/json?address="+address+",+"+city+",+"+state+",+"+zip+"&sensor=false", " ", "+", -1))
	if err != nil {
		log.Fatal(err)
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	if res.StatusCode != 200 {
		log.Fatal("Unexpected status code", res.StatusCode)
	}
	data := map[string]interface{}{}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	err = dec.Decode(&data)
	if err != nil {
		fmt.Println(err)
	}
	jq := jsonq.NewQuery(data)

	lat, err = jq.Float("results", "0", "geometry", "location", "lat")
	if err != nil {
		fmt.Println(err)
	}

	lng, err = jq.Float("results", "0", "geometry", "location", "lng")
	if err != nil {
		fmt.Println(err)
	}
	return
}

//Handle all requests
func Handler(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-type", "text/html")
	webpage, err := ioutil.ReadFile("index.html")
	if err != nil {
		http.Error(response, fmt.Sprintf("home.html file error %v", err), 500)
	}
	fmt.Fprint(response, string(webpage))
}

func APIHandler(response http.ResponseWriter, request *http.Request) {
	session, err := mgo.Dial("mongodb://sushain:1234@ds043694.mongolab.com:43694/tripdb")
	if err != nil {
		panic(err)
	}
	defer session.Close()

	// Optional. Switch the session to a monotonic behavior.
	session.SetMode(mgo.Monotonic, true)

	c := session.DB("tripdb").C("locations")

	//set mime type to JSON
	response.Header().Set("Content-type", "application/json")

	err = request.ParseForm()
	if err != nil {
		http.Error(response, fmt.Sprintf("error parsing url %v", err), 500)
	}

	var result []Location
	id := strings.Replace(request.URL.Path, "/locations/", "", -1)
	switch request.Method {
	case "GET":
		if id != "" {
			err = c.Find(bson.M{"_id": bson.ObjectIdHex(id)}).All(&result)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			err = c.Find(nil).All(&result)
		}
	case "POST":
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			panic(err)
		}
		var location Location
		err = json.Unmarshal(body, &location)
		if err != nil {
			panic(err)
		}

		lat, lng := getCord(location.Address, location.City, location.State, location.Zip)
		i := bson.NewObjectId()
		location.Id = i

		location.Coordinate = Coordinate{lat, lng}
		log.Println(location)

		err = c.Insert(location)
		if err != nil {
			log.Fatal(err)
		}
		err = c.Find(bson.M{"_id": i}).All(&result)
		if err != nil {
			log.Fatal(err)
		}
	case "PUT":
		body, err := ioutil.ReadAll(request.Body)
		if err != nil {
			panic(err)
		}
		var location Location
		err = json.Unmarshal(body, &location)
		if err != nil {
			panic(err)
		}
		i := bson.ObjectIdHex(id)
		location.Id = i

		lat, lng := getCord(location.Address, location.City, location.State, location.Zip)

		location.Coordinate = Coordinate{lat, lng}
		log.Println(location)

		err = c.Update(bson.M{"_id": i}, location)
		if err != nil {
			log.Fatal(err)
		}
		err = c.Find(bson.M{"_id": i}).All(&result)
		if err != nil {
			log.Fatal(err)
		}
	case "DELETE":
		err = c.Remove(bson.M{"_id": bson.ObjectIdHex(id)})
		if err != nil {
			log.Fatal(err)
		}

	default:
	}

	json, err := json.Marshal(result)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Send the text diagnostics to the client.
	fmt.Fprintf(response, "%v", string(json))

}
func main() {
	port := 8080
	var err string
	portstring := strconv.Itoa(port)

	mux := http.NewServeMux()
	mux.Handle("/locations/", http.HandlerFunc(APIHandler))
	mux.Handle("/", http.HandlerFunc(Handler))

	// Start listing on a given port with these routes on this server.
	log.Print("Listening on port " + portstring + " ... ")
	errs := http.ListenAndServe(":"+portstring, mux)
	if errs != nil {
		log.Fatal("ListenAndServe error: ", err)
	}
}
