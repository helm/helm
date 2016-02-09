package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/aokoli/goutils"
	"github.com/codegangsta/cli"
	dep "github.com/deis/helm-dm/deploy"
	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
	"github.com/kubernetes/deployment-manager/chart"
)

func init() {
	addCommands(deployCmd())
}

func deployCmd() cli.Command {
	return cli.Command{
		Name:    "deploy",
		Aliases: []string{"install"},
		Usage:   "Deploy a chart into the cluster.",
		Action:  func(c *cli.Context) { run(c, deploy) },
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Only display the underlying kubectl commands.",
			},
			cli.BoolFlag{
				Name:  "stdin,i",
				Usage: "Read a configuration from STDIN.",
			},
			cli.StringFlag{
				Name:  "name",
				Usage: "Name of deployment, used for deploy and update commands (defaults to template name)",
			},
			// TODO: I think there is a Generic flag type that we can implement parsing with.
			cli.StringFlag{
				Name:  "properties,p",
				Usage: "A comma-separated list of key=value pairs: 'foo=bar,foo2=baz'.",
			},
			cli.StringFlag{
				// FIXME: This is not right. It's sort of a half-baked forward
				// port of dm.go.
				Name:  "repository",
				Usage: "The default repository",
				Value: "kubernetes/application-dm-templates",
			},
		},
	}
}

func deploy(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		format.Err("First argument, filename, is required. Try 'helm deploy --help'")
		os.Exit(1)
	}

	props, err := parseProperties(c.String("properties"))
	if err != nil {
		format.Err("Failed to parse properties: %s", err)
		os.Exit(1)
	}

	d := &dep.Deployment{
		Name:       c.String("Name"),
		Properties: props,
		Filename:   args[0],
		Imports:    args[1:],
		Repository: c.String("repository"),
	}

	if c.Bool("stdin") {
		d.Input = os.Stdin
	}

	return doDeploy(d, c.GlobalString("host"), c.Bool("dry-run"))
}

func doDeploy(cfg *dep.Deployment, host string, dry bool) error {
	if cfg.Filename == "" {
		return errors.New("A filename must be specified. For a tar archive, this is the name of the root template in the archive.")
	}

	fi, err := os.Stat(cfg.Filename)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		format.Info("Chart is directory")
		c, err := chart.LoadDir(cfg.Filename)
		if err != nil {
			return err
		}
		if cfg.Name == "" {
			cfg.Name = genName(c.Chartfile().Name)
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
			return err
		}
		cfg.Filename = tfile
	} else if cfg.Name == "" {
		n, _, e := parseTarName(cfg.Filename)
		if e != nil {
			return e
		}
		cfg.Name = n
	}

	if dry {
		format.Info("Prepared deploy %q using file %q", cfg.Name, cfg.Filename)
		return nil
	}

	c := dm.NewClient(host)
	return c.DeployChart(cfg.Filename, cfg.Name)
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
