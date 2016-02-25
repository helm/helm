package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aokoli/goutils"
	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/format"
	"github.com/kubernetes/deployment-manager/chart"
)

func uploadChart(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		format.Err("First argument, filename, is required. Try 'helm deploy --help'")
		os.Exit(1)
	}

	cname := c.String("name")
	fname := args[0]

	if fname == "" {
		return errors.New("A filename must be specified. For a tar archive, this is the name of the root template in the archive.")
	}

	_, err := doUpload(fname, cname, c)
	return err
}
func doUpload(filename, cname string, cxt *cli.Context) (string, error) {

	fi, err := os.Stat(filename)
	if err != nil {
		return "", err
	}

	if fi.IsDir() {
		format.Info("Chart is directory")
		c, err := chart.LoadDir(filename)
		if err != nil {
			return "", err
		}
		if cname == "" {
			cname = genName(c.Chartfile().Name)
		}

		// TODO: Is it better to generate the file in temp dir like this, or
		// just put it in the CWD?
		//tdir, err := ioutil.TempDir("", "helm-")
		//if err != nil {
		//format.Warn("Could not create temporary directory. Using .")
		//tdir = "."
		//} else {
		//defer os.RemoveAll(tdir)
		//}
		tdir := "."
		tfile, err := chart.Save(c, tdir)
		if err != nil {
			return "", err
		}
		filename = tfile
	} else if cname == "" {
		n, _, e := parseTarName(filename)
		if e != nil {
			return "", e
		}
		cname = n
	}

	// TODO: Add a version build metadata on the chart.

	if cxt.Bool("dry-run") {
		format.Info("Prepared deploy %q using file %q", cname, filename)
		return "", nil
	}

	c := client(cxt)
	return c.PostChart(filename, cname)
}

func genName(pname string) string {
	s, _ := goutils.RandomAlphaNumeric(8)
	return fmt.Sprintf("%s-%s", pname, s)
}

func parseTarName(name string) (string, string, error) {
	tnregexp := regexp.MustCompile(chart.TarNameRegex)
	if strings.HasSuffix(name, ".tgz") {
		name = strings.TrimSuffix(name, ".tgz")
	}
	v := tnregexp.FindStringSubmatch(name)
	if v == nil {
		return name, "", fmt.Errorf("invalid name %s", name)
	}
	return v[1], v[2], nil
}
