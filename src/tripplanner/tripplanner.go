package tripplanner

import(
  "io"
  "fmt"
  m "encoding/json"
  "io/ioutil"
  "net/http"
  "github.com/julienschmidt/httprouter"
  "gopkg.in/mgo.v2"
  "gopkg.in/mgo.v2/bson"
  "strconv"
  "strings"
  "log"
  "bytes"
)
  //)

type Req struct{
  Name string `json:"name"`
}
type Resp struct{
  Name string `json:"greeting"`
}

type Counter struct{
    Id string
    Location_Id int
}

type TripReq struct{
  Starting_Location string `json:"starting_from_location_id"`
  Locations []string `json:"location_ids"`
}

type TripResp struct{
  Name string `json:"greeting"`
}

type FinalTripResp struct{
  Id string `json:"id"`
  Status string `json:"status"`
  Starting_Location string `json:"starting_from_location_id"`
  Next_Location string `json:"next_destination_location_id,omitempty"`
  BestRoute []string `json:"best_route_location_ids"`
  Cost int `json:"total_uber_costs"`
  Duration int `json:"total_uber_duration"`
  Distance float64 `json:"total_distance"`
}

type RequestCabResp struct{
  Id string `json:"id"`
  Status string `json:"status"`
  Starting_Location string `json:"starting_from_location_id"`
  Next_Location string `json:"next_destination_location_id,omitempty"`
  BestRoute []string `json:"best_route_location_ids"`
  Cost int `json:"total_uber_costs"`
  Duration int `json:"total_uber_duration"`
  Distance float64 `json:"total_distance"`
  Eta int `json:"eta"`
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

type UberResponse struct{
  Prices []PriceDetails `json:"prices"`
}

type UberEtaResponse struct{
  ETA int `json:"eta"`
}

type UberEtaRequest struct{
  ProductId string `json:"product_id"`
  Start_latitude float64 `json:"start_latitude"`
  Start_longitude float64 `json:"start_longitude"`
  End_latitude float64 `json:"end_latitude"`
  End_longitude float64 `json:"end_longitude"`
}

type PriceDetails struct{
  VehicleName string `json:"localized_display_name"`
  Cost int `json:"low_estimate"`
  Duration int `json:"duration"`
  Distance float64 `json:"distance"`
  ProductId string `json:"product_id"`
}

var routeCombinations []string
var costMatrix [][]int
var durationMatrix [][]int
var distanceMatrix [][]float64

func FindTrip(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
  tripId:=p.ByName("trip_id")
  findTripResponse := findTripDB(tripId)
  rw.WriteHeader(http.StatusOK)
  rw.Header().Set("Content-Type", "application/json;charset=UTF-8")
  if err := m.NewEncoder(rw).Encode(findTripResponse); err != nil {
     panic(err)
 }
}

func findTripDB(tripId string) FinalTripResp{
  fmt.Println("Establishing DB connection")
  session := getDBSession()
  tripResp := FinalTripResp{}
  defer func(){
    session.Close()
    if r := recover(); r != nil {
        return
    }
    }()
  session.SetMode(mgo.Monotonic, true)
  locations := session.DB("test_db_273").C("trips")
  fmt.Println("DB connection established")
    err := locations.Find(bson.M{"id": tripId}).One(&tripResp)
    if err != nil {
            panic(err)
    }
    fmt.Println("Database accessed")
    return tripResp
}

func UpdateTrip(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
  var requestCabResp RequestCabResp
  var previousLocation string
  tripId:=p.ByName("trip_id")
  findTripResponse := findTripDB(tripId)
  requestCabResp.Id=findTripResponse.Id
  requestCabResp.Starting_Location=findTripResponse.Starting_Location
  requestCabResp.Next_Location=findTripResponse.Next_Location
  requestCabResp.BestRoute=findTripResponse.BestRoute
  requestCabResp.Cost=findTripResponse.Cost
  requestCabResp.Duration=findTripResponse.Duration
  requestCabResp.Distance=findTripResponse.Distance
  requestCabResp.Status=findTripResponse.Status

  if(requestCabResp.Next_Location==""){
    requestCabResp.Status="Requesting"
    previousLocation=findTripResponse.Starting_Location
    if(len(requestCabResp.BestRoute)>0){
      requestCabResp.Next_Location=requestCabResp.BestRoute[0]
      fmt.Println("next loc value"+requestCabResp.Next_Location)
    }
  }else{
    previousLocation=requestCabResp.Next_Location
    for i,location:= range requestCabResp.BestRoute{
      if(previousLocation==location){
        if(i<(len(requestCabResp.BestRoute)-1)){
            requestCabResp.Status="Requesting"
            requestCabResp.Next_Location=requestCabResp.BestRoute[i+1]
            requestCabResp.Status="Requesting"
            requestCabResp.Next_Location=requestCabResp.BestRoute[i+1]
        }else{
          requestCabResp.Status="Requesting"
          requestCabResp.Next_Location=findTripResponse.Starting_Location
        }
      }
    }
    if (requestCabResp.Next_Location==findTripResponse.Next_Location && findTripResponse.Status=="Requesting"){
        requestCabResp.Status="Completed"
    }
  }
  findTripResponse.Status=requestCabResp.Status
  findTripResponse.Next_Location=requestCabResp.Next_Location

  eta:=findUberETA(previousLocation,requestCabResp.Next_Location)
  requestCabResp.Eta=eta
  updateTripDB(findTripResponse,tripId)
  rw.WriteHeader(http.StatusCreated)
  if err := m.NewEncoder(rw).Encode(requestCabResp); err != nil {
   panic(err)
  }
 }

func findUberETA(startLocation string, endLocation string)(int){
  fmt.Println("start from: "+startLocation)
  fmt.Println("start from: "+endLocation)
  startLoc,_:=strconv.Atoi(startLocation)
  endLoc,_:=strconv.Atoi(endLocation)
  start_lat,start_lng:=findLatLng(startLoc)
  end_lat,end_lng:=findLatLng(endLoc)
  _,_,_,productId:=callUberPriceAPI(start_lat,start_lng,end_lat,end_lng)
  eta:= callUberRequestAPI(start_lat,start_lng,end_lat,end_lng,productId)
  return eta
}

func callUberRequestAPI(startLat float64, startLng float64, endLat float64, endLng float64,productId string)(int){
  uberEtaRequest := UberEtaRequest{}
  uberEtaRequest.Start_latitude = startLat
  uberEtaRequest.Start_longitude = startLng
  uberEtaRequest.End_latitude = endLat
  uberEtaRequest.End_longitude = endLng
  uberEtaRequest.ProductId=productId
  uberEtaRequestJSON, err := m.Marshal(uberEtaRequest)
  if err!=nil{
    fmt.Println("error in marshalling line 221")
  }
  url:="https://sandbox-api.uber.com/v1/requests"
  req, err := http.NewRequest("POST", url, bytes.NewBuffer(uberEtaRequestJSON))
  if err!=nil{
    fmt.Println("error in post request line 226")
  }
  // O Auth token here
  req.Header.Add("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsiaGlzdG9yeSIsInByb2ZpbGUiLCJyZXF1ZXN0IiwiaGlzdG9yeV9saXRlIiwiZGVsaXZlcnkiLCJyZXF1ZXN0X3JlY2VpcHQiLCJkZWxpdmVyeV9zYW5kYm94Il0sInN1YiI6IjBhM2RmZmU4LWM0ZDYtNDkyMS04Mzc1LTE5Mjk5ODczOGUwNSIsImlzcyI6InViZXItdXMxIiwianRpIjoiNDljYjE3OWItZjMzZS00NTcwLThiMWItZTkzNDk2YTM3YmE5IiwiZXhwIjoxNDUwNzUxMTU5LCJpYXQiOjE0NDgxNTkxNTgsInVhY3QiOiI1d3hyc0JTSnNxUXowVzlTQkEwMTl6NFpEOHVoeU0iLCJuYmYiOjE0NDgxNTkwNjgsImF1ZCI6IktEUGxfOGJVZkkzbFZjcWJpV3NhcS1XbXVvMlBYTGJnIn0.Yu6UervwkAKZnGbAXy3HQrZNALdQr2Fc46wjUtXCOS0j9z507hlTn-vdmUHB4MJTcYPjIFsgxGg8zYNw7rkk4COZBBuR-8LVZHyQ34qx4QsymCZeCuxdXrqwL5O7YdXDJ6xommOy6n70Ya5kY48FmzM5Zbt0hkutO0CUV_5cQ_vvjgNpfigJ_nhxxFlay4M_vwfk8Kdqc1IRk0zicS1cgAzZPgtHTpZ3lxb94kSh___at6ZiDg7qalPZZr4EFqKHo4KRyOMXMXK4ckyF7ZRunn1mcYohr3iHqiiZNAZmpp56u_VUTLIEUonP_OhhNTzj1jVcyDUEp9Rjcv7xENEXlA")
  req.Header.Add("Content-Type", "application/json")
  client := &http.Client{}
  response, err := client.Do(req)
    if(err!=nil){
      panic(err)
    }
    body, err := ioutil.ReadAll(response.Body)
    response.Body.Close()
    if(err!=nil){
      fmt.Println("error at 239 response body")
    }
    var uberETAResponse UberEtaResponse
    err=m.Unmarshal(body, &uberETAResponse)
    if(err!=nil){
      fmt.Println("partial unmarshal")
    }
    eta:=uberETAResponse.ETA
    return eta
}

func updateTripDB(findTripResponse FinalTripResp,tripId string)(string){
  session := getDBSession()
  defer func(){
    session.Close()
    if r := recover(); r != nil {
        return
    }
    }()
    trips := session.DB("test_db_273").C("trips")
    err := trips.Update(bson.M{"id": tripId},bson.M{"$set": findTripResponse})
    if err != nil{
      panic(err)
    }
    return "Resource updated successfully"
}

func AddTrip(rw http.ResponseWriter, tripreq *http.Request , p httprouter.Params) {
    body,_ := ioutil.ReadAll(io.LimitReader(tripreq.Body, 1048576))
    request:=&TripReq{ }
    m.Unmarshal(body, &request)
    startLocation:=request.Starting_Location
    costMatrixcount := len(request.Locations)+1
    count := len(request.Locations)
    locations:=request.Locations
    var locForCostMatrix []string
    locForCostMatrix=append([]string{startLocation},locForCostMatrix...)
    locForCostMatrix=append(locForCostMatrix,locations...)

    costMatrix = make([][]int, costMatrixcount)
    durationMatrix = make([][]int, costMatrixcount)
    distanceMatrix = make([][]float64, costMatrixcount)
    for i := range costMatrix {
	     costMatrix[i] = make([]int, costMatrixcount)
    }
    for i := range durationMatrix {
       durationMatrix[i] = make([]int, costMatrixcount)
    }
    for i := range distanceMatrix {
	     distanceMatrix[i] = make([]float64, costMatrixcount)
    }
    i:=0
    for i<costMatrixcount{
        startLoc,_:=strconv.Atoi(locForCostMatrix[i])
        start_lat,start_lng:=findLatLng(startLoc)
        j:=0
        for j<costMatrixcount{
          endLoc,_:=strconv.Atoi(locForCostMatrix[j])
          if(startLoc!=endLoc){
            end_lat,end_lng:=findLatLng(endLoc)
            Cost,Duration,Distance := findMetrics(start_lat,start_lng,end_lat,end_lng)
            costMatrix[i][j]=Cost
            durationMatrix[i][j]=Duration
            distanceMatrix[i][j]=Distance
          }else{
            costMatrix[i][j]=0
            durationMatrix[i][j]=0
            distanceMatrix[i][j]=0.0
          }
          j+=1
      }
      i+=1
    }
    calculatePossibleRoutes(locations,count, startLocation)
    for i,val:=range routeCombinations{
      val="0"+","+val+","+"0"
      routeCombinations[i]=val
    }
    bestroute,bestRouteCost,bestRouteDuration,bestRouteDistance:=calculateBestRoute(routeCombinations,costMatrix,durationMatrix,distanceMatrix)
    bestRouteLocations:=findBestRouteLocatioId(bestroute,locForCostMatrix)
    var bestRouteLocationsResp []string
    bestLocRoute:=""
    for i,val:= range bestRouteLocations{
      bestLocRoute+=val+","
      if(i>0 && i<(len(bestRouteLocations)-1)){
        bestRouteLocationsResp=append(bestRouteLocationsResp,val)
      }
    }

    var finalTripResp FinalTripResp
    finalTripResp.Status = "planning"
    finalTripResp.Starting_Location=request.Starting_Location
    finalTripResp.BestRoute =bestRouteLocationsResp
    finalTripResp.Cost=bestRouteCost
    finalTripResp.Duration=bestRouteDuration
    finalTripResp.Distance=bestRouteDistance
    finalTripResp.Next_Location=""

    addTripResponse:=addTripDb(finalTripResp)
    rw.Header().Set("Content-Type", "application/json;charset=UTF-8")
    rw.WriteHeader(http.StatusCreated)
    if err := m.NewEncoder(rw).Encode(addTripResponse); err != nil {
       panic(err)
   }
}

func addTripDb(finalTripResp FinalTripResp) (FinalTripResp){
  session := getDBSession()
  defer session.Close()
  counters:= session.DB("test_db_273").C("tripcounters")
  trips := session.DB("test_db_273").C("trips")
  change := mgo.Change{
          Update: bson.M{"$inc": bson.M{"location_id": 1}},
          ReturnNew: true,
  }
  counter:=Counter{}
  counters.Find(bson.M{"id": "count"}).Apply(change, &counter)
  tripId := counter.Location_Id
  finalTripResp.Id = strconv.Itoa(tripId)
  err := trips.Insert(finalTripResp)
    if err != nil {
          log.Fatal(err)
  }
  return finalTripResp
}

func findBestRouteLocatioId(bestRoute string,locForCostMatrix []string) ([]string){
prefarraysplit := strings.Split(bestRoute,",");
var bestRouteArray []string
  for _,val:=range prefarraysplit{
    routeIndex,_:=strconv.Atoi(val)
    bestRouteArray=append(bestRouteArray,locForCostMatrix[routeIndex])
  }
  return bestRouteArray
}

func calculateBestRoute(routeCombinations []string,costMatrix [][]int,durationMatrix [][]int, distanceMatrix[][]float64) (string,int,int,float64){
type routeDetails struct{
   cost int
   duration int
   route string
   distance float64
}
minimumCost:=2147483647
minimumDuration:=2147483647
minimumDistance:=0.0
bestRoute:=""
routesDetails:=make([]routeDetails,len(routeCombinations))

  for routeIndex,currentRoute:=range routeCombinations{
    var currentRouteDetails routeDetails
    routeCost:=0
    routeDuration:=0
    routeDistance:=0.0
    prefarraysplit := strings.Split(currentRoute,",");
    for i,_:=range prefarraysplit{
        if(i+1<=len(prefarraysplit)-1){
          startLocationIndex,_:=strconv.Atoi(prefarraysplit[i])
          endLocationIndex,_:=strconv.Atoi(prefarraysplit[i+1])
            routeCost+=costMatrix[startLocationIndex][endLocationIndex]
            routeDuration+=durationMatrix[startLocationIndex][endLocationIndex]
            routeDistance+=distanceMatrix[startLocationIndex][endLocationIndex]
          }else{
            startLocationIndex,_:=strconv.Atoi(prefarraysplit[i])
            endLocationIndex,_:=strconv.Atoi(prefarraysplit[0])
            routeCost+=costMatrix[startLocationIndex][endLocationIndex]
            routeDuration+=durationMatrix[startLocationIndex][endLocationIndex]
            routeDistance+=distanceMatrix[startLocationIndex][endLocationIndex]
          }
      }
      currentRouteDetails.cost=routeCost
      currentRouteDetails.duration=routeDuration
      currentRouteDetails.route=currentRoute
      currentRouteDetails.distance=routeDistance

      // find best route
      if(routeCost<minimumCost){
        minimumCost=routeCost
        minimumDuration=routeDuration
        bestRoute=currentRoute
        minimumDistance=routeDistance
      }else if routeCost==minimumCost{
        if(routeDuration<minimumDuration){
          minimumCost=routeCost
          minimumDuration=routeDuration
          bestRoute=currentRoute
          minimumDistance=routeDistance
        }
      }
      routesDetails[routeIndex]=currentRouteDetails
    }

    return bestRoute,minimumCost,minimumDuration,minimumDistance
}

func calculatePossibleRoutes(locations []string ,count int, startLocation string){
  routeCombinations = routeCombinations[:0]
  findRouteCombinationsRec(locations,"",count,count)
}

func findRouteCombinationsRec(locations []string,prefix string,numOfLocations int,count int){
  // Base case
  if(count==0){
    routeCombinations=append(routeCombinations,prefix)
    return;
  }

  i:=0
  for i<numOfLocations{
    j:=strconv.Itoa(i+1)
    prefarraysplit := strings.Split(prefix,",");
    prefixMap:=make(map[string] struct{},len(prefarraysplit))
    for _,s:=range prefarraysplit{
      prefixMap[s]=struct{}{}
    }
    if _,ok :=prefixMap[j]; !ok{
      newPrefix:=""
      if(prefix!=""){
        newPrefix=prefix+","+j
      }else{
        newPrefix=prefix+j
      }
      findRouteCombinationsRec(locations,newPrefix,numOfLocations,count-1)
    }
    i++
  }
}


func findMetrics(startLat float64, startLng float64, endLat float64, endLng float64)(int,int,float64){
    cost,duration,distance,_:=callUberPriceAPI(startLat,startLng,endLat,endLng)
  return cost,duration,distance
}

func findLatLng(startLocation int)(float64,float64){
  coord:= map[string]interface{}{}
  session := getDBSession()
  defer session.Close()
  locations := session.DB("test_db_273").C("locations")
  locations.Find(bson.M{"id": startLocation}).Select(bson.M{"coordinate":1}).One(&coord)
  coordinate:=coord["coordinate"]
  b:=coordinate.(map[string]interface{})
  lat:=b["lat"].(float64)
  lng:=b["lng"].(float64)
  return lat,lng
}

func callUberPriceAPI(startLat float64, startLng float64, endLat float64, endLng float64)(int,int,float64,string){
startLatStr := strconv.FormatFloat(startLat, 'f',7, 64)
startLngStr := strconv.FormatFloat(startLng, 'f',7, 64)
endLatStr := strconv.FormatFloat(endLat,'f',7,64)
endLngStr := strconv.FormatFloat(endLng,'f',7,64)
url:="https://sandbox-api.uber.com/v1/estimates/price?start_latitude="+startLatStr+"&start_longitude="+startLngStr+"&end_latitude="+endLatStr+"&end_longitude="+endLngStr+"&server_token=lYW85oaYLuYqcNjtsqBlcCgkq2DMEgPBlpGhnF5y"
response,err:=http.Get(url)
  if(err!=nil){
    panic(err)
  }
  body, err := ioutil.ReadAll(response.Body)
  uberEstimateResponse:=&UberResponse{}
  m.Unmarshal(body, &uberEstimateResponse)
  minCost:=2147483647
  var duration int
  var distance float64
  var productId string
  for _,uberEstimate:=range uberEstimateResponse.Prices{
    //  if(uberEstimate.Cost!=0 && uberEstimate.Cost<minCost){
      if(uberEstimate.VehicleName=="uberX"){
        minCost=uberEstimate.Cost
        duration=uberEstimate.Duration
        distance=uberEstimate.Distance
        productId=uberEstimate.ProductId
        break
      }
  }
  defer func() {
          response.Body.Close()
         if r := recover(); r != nil {
             fmt.Sprintf("uber api call failed",r)
             return
         }
     }()
  return minCost,duration,distance,productId
}
func getDBSession() mgo.Session{
  session, err := mgo.Dial("mongodb://vasu:password@ds063833.mongolab.com:63833/test_db_273")
  if err != nil {
          panic(err)
  }
  session.SetMode(mgo.Monotonic, true)
  return *session
}
