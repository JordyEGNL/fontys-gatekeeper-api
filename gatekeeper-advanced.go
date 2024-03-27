//
// Description: Gatekeeper advanced with JSON file
// Sources:
// https://tutorialedge.net/golang/parsing-json-with-golang/
// https://betterprogramming.pub/parsing-and-creating-yaml-in-go-crash-course-2ec10b7db850
// https://stackoverflow.com/questions/34647039/how-to-use-fmt-scanln-read-from-a-string-separated-by-spaces
// https://pkg.go.dev/fmt#Scanf
// https://tutorialedge.net/golang/golang-mysql-tutorial/
// https://git.fhict.nl/I882775/slagboom_app/-/blob/main/main.go#L17-L24
//
// Made by: Jordy Hoebergen
// Date: 2024-03-18
//

package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// Function used to log debug information when debug mode is enabled
func debug(s string) {
	if config.Global.Debug {
		log.Println("DEBUG: " + s)
	}
}

// Init function to connect to load config and connect to db
func init() {
	// Create a new log file or open the existing log file in append mode (add new lines)
	logFile, err := os.OpenFile("trace.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Couldn't create logfile")
	}

	writer := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(writer)

	// Read the config file
	readYaml()

	// Check DB connection
	db, err := initializeDB()
	if err != nil {
		log.Printf("WARN: Cannot connect to the database: %v", err)
	}
	defer db.Close()
}

// Create a struct to store the visitors
// Used to convert the data to JSON
type visitors struct {
	Name  string `json:"name"`
	Plate string `json:"plate"`
}

func main() {
	// Create a new instance of the gin router
	router := gin.Default()

	// Create a new route for the APIs
	router.GET("/visitors", getVisitors)
	router.GET("/visitors/:plate", getVisitors)
	router.POST("/visitors", addVisitor)
	router.DELETE("/visitors/:plate", removeVisitor)
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// Run the server
	router.Run("0.0.0.0:8080")

	// readAllowedPlates()
	//startProgramMessage()
	// scanPlate()
	// pressKeyToContinue()
}

// This function gets called by the API
// GET to /visitors
//
// Example response:
//
//	[
//		{
//			"name": "Jordy",
//			"plate": "ABC-123"
//		},
//		{
//			"name": "Piet",
//			"plate": "DEF-456"
//		}
//	]
func getVisitors(c *gin.Context) {
	// Get the plate from the URL
	// Example: /visitors/ABC-123
	plate := c.Param("plate")
	results := getVisitorsFromDB(plate)
	if results == nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Plate is not found in the database"})
		return
	}
	c.JSON(http.StatusOK, results)
}

// This function gets called by the API
// POST to /visitors
//
// Example request body:
//
//	{
//		"name": "Jordy",
//		"plate": "ABC-123"
//	}
func addVisitor(c *gin.Context) {

	// Bind the JSON data to the newVisitor struct
	var newVisitor visitors
	if err := c.BindJSON(&newVisitor); err != nil {
		return
	}

	// Check if the name and plate are given
	if newVisitor.Name == "" || newVisitor.Plate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Name and plate are required"})
		return
	}

	// Initialize the database
	db, err := initializeDB()
	if err != nil {
		log.Fatalf("ERROR: Cannot connect to the database: %v", err)
	}
	defer db.Close()

	// Check if the plate is already in the database
	if checkPlateAlreadyInDB(db, newVisitor.Plate) {
		c.JSON(http.StatusConflict, gin.H{"message": "Plate already in database"})
		return
	}

	// Add the new visitor to the database
	if addNewVisitorToDB(newVisitor); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}
	c.JSON(http.StatusCreated, newVisitor)
}

func removeVisitor(c *gin.Context) {
	// Get the plate from the URL
	// Example: /visitors/ABC-123
	plate := c.Param("plate")

	db, err := initializeDB()
	if err != nil {
		log.Fatalf("ERROR: Cannot connect to the database: %v", err)
	}
	defer db.Close()

	// Check is in the DB
	if !checkPlateAlreadyInDB(db, plate) {
		c.JSON(http.StatusConflict, gin.H{"message": "Plate is not found in the database"})
		log.Printf("INFO: Tried to delete plate %s but it is not found in the database", plate)
		return
	}

	query := `DELETE FROM visitors
		WHERE plate = '` + plate + `'`
	_, err = db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plate removed"})
	log.Printf("INFO: Plate %s removed from the database", plate)
}

// Function that connect to DB and adds a new visitor
// Returns an error if something went wrong
// Not checking if users exists, this should be done before calling this function
//
// Example:
//
//	addNewVisitorToDB(visitors{Name: "Jordy", Plate: "ABC-123"})
func addNewVisitorToDB(visitor visitors) {
	db, err := initializeDB()
	if err != nil {
		log.Fatalf("ERROR: Cannot connect to the database: %v", err)
	}
	defer db.Close()

	query := `INSERT INTO visitors (name, plate)
		VALUES (?, ?)`
	_, err = db.Query(query, visitor.Name, visitor.Plate)
	if err != nil {
		log.Fatalf("ERROR: Cannot insert new visitor: %v", err)
	}
	log.Printf("INFO: Visitor %s with plate %s added to the database", visitor.Name, visitor.Plate)
}

// Function to get all visitors from the database
// Returns a slice of visitors
//
// Example response:
//
//	[
//		{
//			"name": "Jordy",
//			"plate": "ABC-123"
//		},
//		{
//			"name": "Piet",
//			"plate": "DEF-456"
//		}
//	]
func getVisitorsFromDB(EnteredPlate string) []visitors {
	db, err := initializeDB()
	if err != nil {
		log.Fatalf("ERROR: Cannot connect to the database: %v", err)
	}
	defer db.Close()

	// If the plate is given, get the visitor with the given plate
	rows := &sql.Rows{}

	if EnteredPlate == "" {
		query := `SELECT name, plate
			FROM visitors`
		rows, _ = db.Query(query)
	} else {
		query := `SELECT name, plate
			FROM visitors
			WHERE plate = ?`
		rows, _ = db.Query(query, EnteredPlate)

	}

	var name, plate string
	var visitorList []visitors
	for rows.Next() {
		err := rows.Scan(&name, &plate)
		if err != nil {
			log.Fatalf("ERROR: Cannot scan the row: %v", err)
		}
		visitor := visitors{
			Name:  name,
			Plate: plate,
		}
		visitorList = append(visitorList, visitor)

		// Check if the list is empty
		if len(visitorList) == 0 {
			return nil
		}
	}
	return visitorList
}

// Create a struct to store the configuration data
//
// Example:
//
//	global:
//		debug: true
//	database:
//		user: "username"
//		password: "your password"
//		host: "mydb.local"
//		port: "3306"
//		database: "dbname"
type Config struct {
	// Create a struct to store the database configuration data
	// `yaml:"database" will link the value to the key in the yaml file
	Database struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		Database string `yaml:"database"`
	} `yaml:"database"`
	Global struct {
		Debug bool `yaml:"debug"`
	} `yaml:"global"`
}

// Create a new variable of type Config
// Used by:
// readYaml - to get the configuration data
// writeYaml - to create a new configuration file
// initializeDB - to connect to the database with given configuration data
var config Config

func readYaml() {
	// If the file does not exist, create a new file
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		writeYaml()
		log.Fatalf("Config file does not exist. A new config file has been created. Please fill in the configuration data and restart the program.")
	}

	// Read the yaml file and store it in the 'yamlFile' variable
	yamlFile, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Cannot read the YAML-file: %v", err)
	}

	// Decode the YAML data and store it in the 'config' variable
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		log.Fatalf("Cannot decode YAML-file: %v", err)
	}
}

func writeYaml() {
	// Create a new instance of the Config struct
	config := Config{
		Database: struct {
			User     string `yaml:"user"`
			Password string `yaml:"password"`
			Host     string `yaml:"host"`
			Port     string `yaml:"port"`
			Database string `yaml:"database"`
		}{
			User:     "",
			Password: "",
			Host:     "",
			Port:     "",
			Database: "",
		},
		Global: struct {
			Debug bool `yaml:"debug"`
		}{
			Debug: false,
		},
	}

	// Marshal the data to a YAML file
	// Converting the data to a YAML format
	yamlData, err := yaml.Marshal(&config)
	if err != nil {
		log.Fatalf("Cannot marshal data: %v", err)
	}

	// Write the data to the file
	err = os.WriteFile("config.yaml", yamlData, 0644)
	if err != nil {
		log.Fatalf("Cannot write to file: %v", err)
	}

	fmt.Println("Config file created successfully.")
}

// Function to connect to the database
// Returns a pointer to the database
func initializeDB() (*sql.DB, error) {
	return nil, nil
}

// First message that will be shown to the user
func startProgramMessage() {
	if config.Global.Debug {
		fmt.Println("!! Debug mode is enabled !!")
	}
	fmt.Println("----------------------")
	fmt.Println("Fonteyn Vakantieparken")
	fmt.Println("----------------------")
	fmt.Println("")
	fmt.Println("Opties:")
	fmt.Println("1. Scan kenteken")
	fmt.Println("2. Beheer")
	fmt.Println("3. Afsluiten")
	fmt.Println("")
	fmt.Print("Kies een optie: ")
	var option int
	fmt.Scanln(&option)
	switch option {
	case 1:
		scanPlate()
	case 2:
		startManagementMessage()
	case 3:
		os.Exit(0)
	default:
		startProgramMessage()
	}
}

// Function to start the management message
func startManagementMessage() {
	fmt.Println("----------------------")
	fmt.Println("Fonteyn Vakantieparken")
	fmt.Println("      MANAGEMENT      ")
	fmt.Println("----------------------")
	fmt.Println("")
	fmt.Println("Opties:")
	fmt.Println("1. Lijst kentekens")
	fmt.Println("2. Voeg kenteken toe")
	fmt.Println("3. Verwijder kenteken")
	fmt.Println("4. Terug")
	fmt.Println("")
	fmt.Print("Kies een optie: ")
	var option int
	fmt.Scanln(&option)
	switch option {
	case 1:
		showAllPlates()
	case 2:
		addNewPlate()
	case 3:
		removePlate()
	case 4:
		startProgramMessage()
	default:
		startManagementMessage()
	}
}

// Function to check if the scanned plate is in the database
// Returns a boolean value
// If the plate is in the database, it will return true
// If the plate is not in the database, it will return false
func checkScannedPlateInDB(db *sql.DB, givenPlate string) bool {
	debug("Entered plate: " + givenPlate)
	query := `SELECT name, plate 
		FROM visitors
		WHERE plate ` + " = '" + givenPlate + "'"
	rows, _ := db.Query(query)
	var name, plate string

	// Loop through the rows
	// check if the given plate is in the database
	for rows.Next() {
		err := rows.Scan(&name, &plate)
		if err != nil {
			log.Fatalf("ERROR: Cannot scan the row: %v", err)
		}
		debug("Returned data from query: " + name + " " + plate)
		if plate == givenPlate {
			return true
		}
	}
	return false
}

func pressKeyToContinue() {
	fmt.Println("Press enter to continue...")
	fmt.Scanln()
}

// In this function we will print a welcome message based on the current hour
func firstMessage() {
	// Get the current hour
	hour := time.Now().Hour()

	// Check the current hour and print a welcome message
	switch {
	case hour >= 7 && hour < 12:
		welcomeMessage("Goedemorgen!")
	case hour >= 12 && hour < 18:
		welcomeMessage("Goedemiddag!")
	case hour >= 18 && hour < 23:
		welcomeMessage("Goedenavond!")
	default:
		fmt.Println("Sorry, de parkeerplaats is ’s nachts gesloten")
	}
}

// In this function we will print a welcome message to the user
func welcomeMessage(n string) {
	println(n + " Welkom bij Fonteyn Vakantieparken")
}

// In this function we will scan the license plate
// It asks the user to enter plate number
func scanPlate() {
	var plate string
	fmt.Print("Voer uw kenteken in: ")
	fmt.Scanln(&plate)

	db, err := initializeDB()
	if err != nil {
		log.Fatalf("ERROR: Cannot connect to the database: %v", err)
	}
	defer db.Close()
	if !checkScannedPlateInDB(db, plate) {
		fmt.Println("Kenteken niet toegestaan")
		log.Printf("INFO: Kenteken %s is niet toegelaten", plate)
		return
	}
	log.Printf("INFO: Kenteken %s is doorgelaten", plate)
	firstMessage()
}

// Function to get the linked name to the given plate
// Returns a string with the linked name
func getLinkedNameOfPlate(plate string) string {
	db, err := initializeDB()
	if err != nil {
		log.Fatalf("ERROR: Cannot connect to the database: %v", err)
	}
	defer db.Close()

	query := `SELECT name, plate
		FROM visitors
		WHERE plate = '` + plate + `'`
	rows, _ := db.Query(query)
	var name, plateDB string
	for rows.Next() {
		err := rows.Scan(&name, &plateDB)
		if err != nil {
			log.Fatalf("ERROR: Cannot scan the row: %v", err)
		}
	}
	return name
}

// Function to show all plates in the database
// This function will print all plates
func showAllPlates() {
	db, err := initializeDB()
	if err != nil {
		log.Fatalf("ERROR: Cannot connect to the database: %v", err)
	}
	defer db.Close()

	query := `SELECT name, plate
		FROM visitors`
	rows, _ := db.Query(query)
	var name, plate string
	fmt.Println("Lijst kentekens:")
	for rows.Next() {
		err := rows.Scan(&name, &plate)
		if err != nil {
			log.Fatalf("ERROR: Cannot scan the row: %v", err)
		}
		fmt.Println(name + " " + plate)
	}
	pressKeyToContinue()
	startManagementMessage()
}

// Function to check if the plate is already in the database
// Returns a boolean value
// false = not in the database
// true = in the database
func checkPlateAlreadyInDB(db *sql.DB, givenPlate string) bool {
	debug("Entered plate: " + givenPlate)
	query := `SELECT name, plate
		FROM visitors
		WHERE plate ` + " = '" + givenPlate + "'"
	rows, _ := db.Query(query)
	var name, plate string

	// Loop through the rows
	// check if the given plate is in the database
	for rows.Next() {
		err := rows.Scan(&name, &plate)
		if err != nil {
			log.Fatalf("ERROR: Cannot scan the row: %v", err)
		}
		debug("Returned data from query: " + name + " " + plate)
		if plate == givenPlate {
			return true
		}
	}
	return false
}

// Function to add a new plate to the database
// This function will ask for the name and plate number
func addNewPlate() {
	db, err := initializeDB()
	if err != nil {
		log.Fatalf("ERROR: Cannot connect to the database: %v", err)
	}
	defer db.Close()

	// Ask for the name and plate number
	// using scanner to accept spaces
	fmt.Print("Naam: ")
	var name string
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		name = scanner.Text()
	}

	fmt.Print("Kenteken: ")
	var plate string
	fmt.Scanln(&plate)
	fmt.Println("Ingevoerde naam: " + name)
	fmt.Println("Ingevoerd kenteken: " + plate)
	fmt.Print("Is dit correct? (j/N): ")
	var correct string
	fmt.Scanln(&correct)

	// Ask if correct, else go back to menu
	if correct != "j" {
		startManagementMessage()
	}

	// Check if the plate is already in the database
	// If linked to a name ask to overwrite
	if checkPlateAlreadyInDB(db, plate) {
		fmt.Printf("Kenteken %s bestaat al, deze is van %s, wil je deze overschrijven? (j/N): ", plate, getLinkedNameOfPlate(plate))
		var correct string
		fmt.Scanln(&correct)
		if correct != "j" {
			fmt.Printf("Kenteken %s niet overschreven", plate)
			time.Sleep(3 * time.Second)
			startManagementMessage()
		}

		// Replace the name with given name
		query := `UPDATE visitors
				SET name = '` + name + `'
				WHERE plate = '` + plate + `'`
		// Execute the query
		_, err = db.Query(query)
		if err != nil {
			log.Printf("ERROR: Cannot update plate: %v", err)
			fmt.Print("Opnieuw proberen? (j/N): ")
			var correct string
			fmt.Scanln(&correct)
			if correct != "j" {
				time.Sleep(3 * time.Second)
				startManagementMessage()
			}
			addNewPlate()
		}

		log.Printf("INFO: Kenteken %s staat nu op naam van %s", plate, name)
		time.Sleep(3 * time.Second)
		startManagementMessage()
	}

	query := `INSERT INTO visitors (name, plate)
		VALUES ('` + name + `', '` + plate + `')`
	_, err = db.Query(query)
	if err != nil {
		log.Fatalf("ERROR: Cannot insert new plate: %v", err)
	}
	log.Printf("INFO: Kenteken %s toegevoegd onder naam van %s", plate, name)
	time.Sleep(3 * time.Second)
	startManagementMessage()
}

func removePlate() {
	db, err := initializeDB()
	if err != nil {
		log.Fatalf("ERROR: Cannot connect to the database: %v", err)
	}
	defer db.Close()

	fmt.Print("Kenteken: ")
	var plate string
	fmt.Scanln(&plate)
	fmt.Println("Ingevoerd kenteken: " + plate)
	fmt.Println("Dit kenteken hoort bij: " + getLinkedNameOfPlate(plate))
	fmt.Print("Is dit correct? (j/N): ")
	var correct string
	fmt.Scanln(&correct)
	if correct != "j" {
		removePlate()
	}

	query := `DELETE FROM visitors
		WHERE plate = '` + plate + `'`
	_, err = db.Query(query)
	if err != nil {
		log.Fatalf("ERROR: Cannot remove plate: %v", err)
	}
	log.Printf("INFO: Kenteken %s verwijderd", plate)
	time.Sleep(3 * time.Second)
	startManagementMessage()
}
