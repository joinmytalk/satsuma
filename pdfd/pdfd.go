package main

import (
	"errors"
	"github.com/bitly/go-nsq"
	"github.com/bitly/go-simplejson"
	"github.com/joinmytalk/xlog"
	"github.com/voxelbrain/goptions"
	"io"
	"os"
	"os/exec"
)

func main() {
	options := struct {
		Topic   string `goptions:"--topic, description='Topic'"`
		Channel string `goptions:"--channel, description='Channel'"`
		Lookupd string `goptions:"--lookupd, description='lookupd address'"`
	}{}

	goptions.ParseAndFail(&options)

	r, err := nsq.NewReader(options.Topic, options.Channel)
	if err != nil {
		xlog.Fatalf("Opening reader for %s/%s failed: %v", options.Topic, options.Channel, err)
	}

	r.AddHandler(&Converter{})

	if err := r.ConnectToLookupd(options.Lookupd); err != nil {
		xlog.Errorf("Connecting to %s failed: %v", options.Lookupd, err)
	}

	select {}
}

type Converter struct {
}

func (c *Converter) HandleMessage(message *nsq.Message) error {
	xlog.Debugf("Processing Message %s: %s", message.Id, string(message.Body))

	msg, err := simplejson.NewJson(message.Body)
	if err != nil {
		xlog.Errorf("HandleMessage: parsing message %s failed: %v", message.Id, err)
		return err
	}

	srcFile := msg.Get("src_file").MustString()
	targetFile := msg.Get("target_file").MustString()

	if _, err := os.Stat(targetFile); err == nil {
		xlog.Debugf("target file %s already exists.", targetFile)
		return nil
	}

	if err := ConvertFileToPDF(srcFile, targetFile); err != nil {
		xlog.Errorf("Converting %s to %s failed: %v", srcFile, targetFile, err)
		os.Remove(srcFile)
		os.Remove(targetFile)
		return nil
	}

	return nil
}

func ConvertFileToPDF(src, target string) error {
	cmd := exec.Command("unoconv", "-f", "pdf", "--stdout", src)
	// cmd := exec.Command("cat", src)
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	defer f.Close()
	stdout, _ := cmd.StdoutPipe()
	err = cmd.Start()
	if err != nil {
		xlog.Errorf("running unoconv failed: %v", err)
		return err
	}
	io.Copy(f, stdout)
	err = cmd.Wait()
	if err != nil {
		xlog.Errorf("cmd.Wait returned error: %v", err)
		return err
	}
	fi, _ := f.Stat()
	if fi.Size() == 0 {
		os.Remove(target)
		xlog.Error("file resulting from conversion is empty")
		return errors.New("empty file")
	}

	return nil
}
