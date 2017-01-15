package main

import (
	"encoding/json"
	_ "encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
)

type config struct {
	Region     string
	Profile    string
	Function   string
	ResultFile string
	CallCount  int
}

func readConfig() (*config, error) {
	conf := &config{}

	bytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		fmt.Println("ReadFile: ", err.Error())
		return nil, err
	}

	if err := json.Unmarshal(bytes, &conf); err != nil {
		fmt.Println("Unmarshal: ", err.Error())
		return nil, err
	}

	return conf, nil
}

func invokeFunction(svc *lambda.Lambda, functionName string, wg *sync.WaitGroup, respChan chan string) {
	defer wg.Done()

	// Call function
	params := &lambda.InvokeInput{
		FunctionName:   aws.String(functionName),
		InvocationType: aws.String("RequestResponse"),
		LogType:        aws.String("None"),
	}

	resp, err := svc.Invoke(params)
	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		fmt.Println(err.Error())
		return
	}

	respChan <- string(resp.Payload)
}

func writeFile(fileName string, respChan chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	fout, err := os.Create(fileName)
	defer fout.Close()
	if err != nil {
		fmt.Println(fileName, err)
		return
	}

	fout.WriteString("[")
	for resp := range respChan {
		fout.WriteString(resp)
		fout.WriteString(",\n")
	}
	fout.WriteString("]")
}

func main() {
	// Read config file
	conf, err := readConfig()
	if err != nil {
		return
	}

	// Init AWS SDK
	config := &aws.Config{
		Region:      aws.String(conf.Region),
		Credentials: credentials.NewSharedCredentials("", conf.Profile),
	}

	sess, err := session.NewSession(config)
	if err != nil {
		fmt.Println("failed to create session,", err)
		return
	}

	svc := lambda.New(sess)

	respChan := make(chan string)
	var fcwg sync.WaitGroup
	fcwg.Add(conf.CallCount)
	for i := 0; i < conf.CallCount; i++ {
		go invokeFunction(svc, conf.Function, &fcwg, respChan)
	}

	var fowg sync.WaitGroup
	fowg.Add(1)
	go writeFile(conf.ResultFile, respChan, &fowg)

	fcwg.Wait()

	close(respChan)

	fowg.Wait()
}
