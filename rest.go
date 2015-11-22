package main

import (
  "fmt"
  "log"
  "github.com/julienschmidt/httprouter"
  "net/http"
  "io/ioutil"
  "io"
  m "encoding/json"
  "strings"
  "gopkg.in/mgo.v2"
  "gopkg.in/mgo.v2/bson"
  "strconv"
  "tripplanner"
)

type AddLocationRequest struct{
  Name string `json:"name"`
  Address string `json:"address"`
  City string `json:"city"`
  State string `json:"state"`
  Zip string `json:"zip"`
}

type LocationResponse struct {
  Id int `json:"id"`
  Name string `json:"name"`
  Address string `json:"address"`
  City string `json:"city"`
  State string `json:"state"`
  Zip string `json:"zip"`
  Coordinate Coord `json:"coordinate"`
}

type Coord struct{
    Lat float64 `json:"lat"`
    Lng float64 `json:"lng"`
}

type Counter struct{
    Id string
    Location_Id int
}

type GoogleResponse struct{
  Results []struct{
    Address_components []struct{
      Long_name string `json:"long_name"`
      Short_name string `json:"short_name"`
    }
    Formatted_address string `json:"formatted_address"`
    Geometry struct {
      Location struct{
        Lat float64 `json:"lat"`
        Lng float64 `json:"lng"`
      }
    }
    PlaceId string `json:"place_id"`
  }
  Status string `json:"status"`
}

func addLocation(rw http.ResponseWriter, req *http.Request , p httprouter.Params) {
    body,_ := ioutil.ReadAll(io.LimitReader(req.Body, 1048576))
    v:=&AddLocationRequest{}
  	m.Unmarshal(body, &v)

    googleResp:= new(GoogleResponse)
    googleResp = callGoogleAPI(v)

    addLocResp:= new(LocationResponse)
	  for _,googResult:=range googleResp.Results{
      addLocResp.Coordinate.Lat = googResult.Geometry.Location.Lat
      addLocResp.Coordinate.Lng = googResult.Geometry.Location.Lng
    }

    addLocResp.Name = v.Name
    addLocResp.Address = v.Address
    addLocResp.City = v.City
    addLocResp.State = v.State
    addLocResp.Zip = v.Zip

    addLocResponse := addLocDB(addLocResp)

    rw.Header().Set("Content-Type", "application/json;charset=UTF-8")
    rw.WriteHeader(http.StatusCreated)
    if err := m.NewEncoder(rw).Encode(addLocResponse); err != nil {
       panic(err)
   }
}

func findLocation(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
  locId,_:=strconv.Atoi(p.ByName("location_id"))
  findLocResponse := findLocDB(locId)
  rw.WriteHeader(http.StatusOK)
  if(findLocResponse.Id==0){
  fmt.Fprintf(rw, "Requested Location not found")
  }else{
  rw.Header().Set("Content-Type", "application/json;charset=UTF-8")
  if err := m.NewEncoder(rw).Encode(findLocResponse); err != nil {
     panic(err)
 }
 }
}

func updateLocation(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
  locId,_:=strconv.Atoi(p.ByName("location_id"))
  findLocResponse := findLocDB(locId)
  rw.WriteHeader(http.StatusCreated)
  if(findLocResponse.Id!=locId){
  fmt.Fprintf(rw, "Requested Location not found")
  }else{
    body,_ := ioutil.ReadAll(io.LimitReader(req.Body, 1048576))
    v:=&AddLocationRequest{}
  	m.Unmarshal(body, &v)
    googleResp:= new(GoogleResponse)
    googleResp = callGoogleAPI(v)
    addLocResp:= new(LocationResponse)
	  for _,googResult:=range googleResp.Results{
      addLocResp.Coordinate.Lat = googResult.Geometry.Location.Lat
      addLocResp.Coordinate.Lng = googResult.Geometry.Location.Lng
    }
    addLocResp.Id = findLocResponse.Id
    addLocResp.Name = findLocResponse.Name
    addLocResp.Address = v.Address
    addLocResp.City = v.City
    addLocResp.State = v.State
    addLocResp.Zip = v.Zip
    resp := updateLocDB(addLocResp,locId)
    if(resp==""){
      fmt.Fprintf(rw, "Requested resource not found")
    }else{
      if err := m.NewEncoder(rw).Encode(addLocResp); err != nil {
         panic(err)
     }
    }
  }
 }

func delLocation(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
  locId,_:=strconv.Atoi(p.ByName("location_id"))
  findLocResponse := delLocDB(locId)
  rw.WriteHeader(http.StatusOK)
  if(findLocResponse==""){
  fmt.Fprintf(rw, "Requested Location not found")
  }else{
  fmt.Fprintf(rw, "Resource deleted successfully")
 }
}


func callGoogleAPI(googReq *AddLocationRequest) *GoogleResponse{
address := strings.Replace(googReq.Address, " ", "+", -1)
if(address!=""){
   address=strings.Replace(address,"&","and",-1)}
city := strings.Replace(googReq.City, " ", "+", -1)
state := strings.Replace(googReq.State, " ", "+", -1)
zip := strings.Replace(googReq.Zip, " ", "+", -1)
url:= "http://maps.google.com/maps/api/geocode/json?address="+address+","+city+","+state+","+zip
fmt.Println(url)
resp,err:= http.Get(url)
if err != nil {
  log.Fatal()
}
defer resp.Body.Close()
body, err := ioutil.ReadAll(resp.Body)
v:=&GoogleResponse{}
m.Unmarshal(body, &v)
return v
}

func main() {
        mux := httprouter.New()
        mux.GET("/locations/:location_id", findLocation)
        mux.POST("/locations", addLocation)
        mux.DELETE("/locations/:location_id",delLocation)
        mux.PUT("/locations/:location_id",updateLocation)
        //mux.POST("/hello",tripplanner.Hellopost)
        mux.POST("/trips",tripplanner.AddTrip)
        mux.GET("/trips/:trip_id",tripplanner.FindTrip)
        mux.PUT("/trips/:trip_id/request",tripplanner.UpdateTrip)
        server := http.Server{
                Addr:        ":8080",
                Handler: mux,
        }
        server.ListenAndServe()
}

func addLocDB(addLocationResp *LocationResponse) LocationResponse{
  session := getDBSession()
  defer session.Close()
  counters:= session.DB("test_db_273").C("counters")
  locations := session.DB("test_db_273").C("locations")
  change := mgo.Change{
          Update: bson.M{"$inc": bson.M{"location_id": 1}},
          ReturnNew: true,
  }
  counter:=Counter{}
  counters.Find(bson.M{"id": "count"}).Apply(change, &counter)
  locationId := counter.Location_Id
  addLocationResp.Id = locationId
  err := locations.Insert(addLocationResp)
  if err != nil {
          log.Fatal(err)
  }
  return *addLocationResp
}

func findLocDB(locId int) LocationResponse{
  fmt.Println("Establishing DB connection")
  session := getDBSession()
  locResp := LocationResponse{}
  defer func(){
    session.Close()
    if r := recover(); r != nil {
        return
    }
    }()
  session.SetMode(mgo.Monotonic, true)
  locations := session.DB("test_db_273").C("locations")
  fmt.Println("DB connection established")
    err := locations.Find(bson.M{"id": locId}).One(&locResp)
    if err != nil {
            panic(err)
    }
    fmt.Println("Database accessed")
    return locResp
}

func delLocDB(locId int) string{
  fmt.Println("Establishing DB connection")
  session := getDBSession()
  defer func(){
    session.Close()
    if r := recover(); r != nil {
        return
    }
    }()
  locations := session.DB("test_db_273").C("locations")
  fmt.Println("DB connection established")
    err := locations.Remove(bson.M{"id": locId})
    if err != nil {
            panic(err)
    }

    fmt.Println("Database accessed")
    return "deleted"
}

func updateLocDB(updateLocationResp *LocationResponse, locID int) string{
  fmt.Println("Establishing DB connection")
  session := getDBSession()
  defer func(){
    session.Close()
    if r := recover(); r != nil {
        return
    }
    }()
    locations := session.DB("test_db_273").C("locations")
    fmt.Println("DB connection established")
    err := locations.Update(bson.M{"id": locID},bson.M{"$set": updateLocationResp})
    if err != nil{
      panic(err)
    }
    return "Resource updated successfully"
}

func getDBSession() mgo.Session{
  session, err := mgo.Dial("mongodb://vasu:password@ds063833.mongolab.com:63833/test_db_273")
  if err != nil {
          panic(err)
  }
  session.SetMode(mgo.Monotonic, true)
  return *session
}
