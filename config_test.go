package main

// import "fmt"
// import "os"
// import "github.com/Jovial-Kanwadia/proxy-server/config"

// func main() {
// 	// Create a default configuration
// 	cfg := config.NewDefaultConfig()
// 	fmt.Println("Default configuration:")
// 	fmt.Println(cfg)
	
// 	// Create a sample configuration file
// 	testConfigFile := "test_config.json"
	
// 	// Modify some values
// 	cfg.Port = 9090
// 	cfg.CacheSize = 2048
// 	cfg.AllowedDomains = []string{"example.com", "google.com"}
	
// 	// Save to file
// 	err := cfg.SaveToFile(testConfigFile)
// 	if err != nil {
// 		fmt.Printf("Error saving config: %v\n", err)
// 		return
// 	}
// 	fmt.Printf("Configuration saved to %s\n", testConfigFile)
	
// 	// Load from file
// 	loadedCfg, err := config.LoadFromFile(testConfigFile)
// 	if err != nil {
// 		fmt.Printf("Error loading config: %v\n", err)
// 		return
// 	}
	
// 	fmt.Println("\nLoaded configuration:")
// 	fmt.Println(loadedCfg)
	
// 	// Validate the configuration
// 	err = loadedCfg.Validate()
// 	if err != nil {
// 		fmt.Printf("Configuration validation error: %v\n", err)
// 		return
// 	}
// 	fmt.Println("Configuration is valid.")
	
// 	// Clean up
// 	os.Remove(testConfigFile)
// 	fmt.Printf("Test config file %s removed.\n", testConfigFile)
// }

