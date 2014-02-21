package main

import (
	"database/sql"
	"errors"
	"github.com/bitly/go-nsq"
	"github.com/bitly/go-simplejson"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joinmytalk/xlog"
	"github.com/voxelbrain/goptions"
	"io"
	"os"
	"os/exec"
)

func main() {
	xlog.SetOutput(os.Stdout)

	options := struct {
		Topic   string `goptions:"--topic, description='Topic', obligatory"`
		Channel string `goptions:"--channel, description='Channel', obligatory"`
		Lookupd string `goptions:"--lookupd, description='lookupd address', obligatory"`
		DSN     string `goptions:"--dsn, description='MySQL DSN string', obligatory"`
	}{}

	goptions.ParseAndFail(&options)

	sqldb, err := sql.Open("mysql", options.DSN)
	if err != nil {
		xlog.Fatalf("sql.Open failed: %v", err)
	}

	r, err := nsq.NewReader(options.Topic, options.Channel)
	if err != nil {
		xlog.Fatalf("Opening reader for %s/%s failed: %v", options.Topic, options.Channel, err)
	}

	r.AddHandler(&Converter{DB: sqldb})

	if err := r.ConnectToLookupd(options.Lookupd); err != nil {
		xlog.Errorf("Connecting to %s failed: %v", options.Lookupd, err)
	}

	select {}
}

type Converter struct {
	DB *sql.DB
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
	publicId := msg.Get("upload_id").MustString()

	if _, err := os.Stat(targetFile); err == nil {
		xlog.Debugf("target file %s already exists.", targetFile)
		return nil
	}

	if err := ConvertFileToPDF(srcFile, targetFile); err != nil {
		xlog.Errorf("Converting %s to %s failed: %v", srcFile, targetFile, err)
		_, err = c.DB.Exec("UPDATE uploads SET conversion = 'error' WHERE public_id = ?", publicId)
		if err != nil {
			xlog.Errorf("Updating conversion status for %s failed: %v", publicId, err)
		}
		os.Remove(srcFile)
		os.Remove(targetFile)
		return nil
	} else {
		_, err = c.DB.Exec("UPDATE uploads SET conversion = 'success' WHERE public_id = ?", publicId)
		if err != nil {
			xlog.Errorf("Updating conversion status for %s failed: %v", publicId, err)
		}
	}
	xlog.Debugf("Conversion of upload %s finished.", publicId)

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
