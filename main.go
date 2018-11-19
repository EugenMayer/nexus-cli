package main

import (
	"fmt"
	"github.com/blang/semver"
	"github.com/eugenmayer/nexus-cli/registry"
	"github.com/eugenmayer/nexus-cli/utils"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
	"html/template"
	"os"
	"strings"
	"syscall"
)

const (
	CredentialTemplate = `# Nexus Credentials
nexus_host = "{{ .Host }}"
nexus_username = "{{ .Username }}"
nexus_password = "{{ .Password }}"
nexus_repository = "{{ .Repository }}"`
)

func main() {
	app := cli.NewApp()
	app.Name = "Nexus CLI"
	app.Usage = "Manage Docker Private Registry on Nexus"
	app.Version = "0.0.2"
	app.Authors = []cli.Author{
		{
			Name:  "Eugen Mayer, Karol Buchta, Mohamed Labouardy",
			Email: "-",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "configure",
			Usage: "Configure Nexus Credentials",
			Action: func(c *cli.Context) error {
				return setNexusCredentials(c)
			},
		},
		{
			Name:  "image",
			Usage: "Manage Docker Images",
			Subcommands: []cli.Command{
				{
					Name:  "ls",
					Usage: "List all images in repository",
					Action: func(c *cli.Context) error {
						return listImages(c)
					},
				},
				{
					Name:  "tags",
					Usage: "Display all image tags",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "name, n",
							Usage: "List tags by image name",
						},
						cli.StringFlag{
							Name:  "sort, s",
							Usage: "Sort tags by semantic version, assuming all tags are semver except latest.",
						},
					},
					Action: func(c *cli.Context) error {
						return listTagsByImage(c)
					},
				},
				{
					Name:  "info",
					Usage: "Show image details",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name: "name, n",
						},
						cli.StringFlag{
							Name: "tag, t",
						},
					},
					Action: func(c *cli.Context) error {
						return showImageInfo(c)
					},
				},
				{
					Name:  "delete",
					Usage: "Delete an image",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name: "name, n",
						},
						cli.StringFlag{
							Name: "tag, t",
						},
						cli.StringFlag{
							Name: "keep, k",
						},
						cli.StringFlag{
							Name: "sort, s",
						},
						cli.BoolFlag{
							Name: "dry-run, d",
						},
					},
					Action: func(c *cli.Context) error {
						return deleteImage(c)
					},
				},
			},
		},
	}
	app.CommandNotFound = func(c *cli.Context, command string) {
		fmt.Fprintf(c.App.Writer, "Wrong command %q !", command)
	}
	app.Run(os.Args)
}

func setNexusCredentials(_ *cli.Context) error {
	var hostname, repository, username, password string
	fmt.Print("Enter Nexus Host: ")
	fmt.Scan(&hostname)
	fmt.Print("Enter Nexus Repository Name: ")
	fmt.Scan(&repository)
	fmt.Print("Enter Nexus Username: ")
	fmt.Scan(&username)
	fmt.Print("Enter Nexus Password: ")
	bytePw, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}

	password = string(bytePw)
	// The password will be read by a toml parser (registry.go)
	// This parser only allows certain escape character sequences and will therefore
	// throw exceptions when your pw contains backslahes in certain cases.
	// Hence we escape all backslash chars again here.
	password = strings.Replace(password, "\\", "\\\\", -1)

	// we need to remove trailing slashes
	hostname = strings.TrimRight(hostname, "/")
	fmt.Printf("Removed potential trailing slash on Nexus Host URL, now: %s\n", hostname)

	data := struct {
		Host       string
		Username   string
		Password   string
		Repository string
	}{
		hostname,
		username,
		password,
		repository,
	}

	tmpl, err := template.New(".credentials").Parse(CredentialTemplate)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	configurationPath := utils.ExpandTildeInPath("~/.nexus-cli")
	f, err := os.Create(configurationPath)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	err = tmpl.Execute(f, data)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	fmt.Printf("Configuration saved to succesfully to: %s\n", configurationPath)
	return nil
}

func listImages(_ *cli.Context) error {
	r, err := registry.NewRegistry()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	images, err := r.ListImages()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	for _, image := range images {
		fmt.Println(image)
	}
	fmt.Printf("Total images: %d\n", len(images))
	return nil
}

func listTagsByImage(c *cli.Context) error {
	var imgName = c.String("name")
	var sort = c.String("sort")
	if sort != "semver" {
		sort = "default"
	}

	r, err := registry.NewRegistry()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if imgName == "" {
		cli.ShowSubcommandHelp(c)
	}
	tags, err := r.ListTagsByImage(imgName)

	compareStringNumber := getSortComparisonStrategy(sort)
	Compare(compareStringNumber).Sort(tags)

	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	for _, tag := range tags {
		fmt.Println(tag)
	}
	fmt.Printf("There are %d images for %s\n", len(tags), imgName)
	return nil
}

func showImageInfo(c *cli.Context) error {
	var imgName = c.String("name")
	var tag = c.String("tag")
	r, err := registry.NewRegistry()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	if imgName == "" || tag == "" {
		cli.ShowSubcommandHelp(c)
	}
	manifest, err := r.ImageManifest(imgName, tag)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	fmt.Printf("Image: %s:%s\n", imgName, tag)
	fmt.Printf("Size: %d\n", manifest.Config.Size)
	fmt.Println("Layers:")
	for _, layer := range manifest.Layers {
		fmt.Printf("\t%s\t%d\n", layer.Digest, layer.Size)
	}
	return nil
}

func deleteImage(c *cli.Context) error {
	var imgName = c.String("name")
	var tag = c.String("tag")
	var keep = c.Int("keep")
	var dryRun = c.Bool("dry-run")
	var sort = c.String("sort")
	if sort != "semver" {
		sort = "default"
	}

	if imgName == "" {
		fmt.Fprintf(c.App.Writer, "You should specify the image name\n")
		cli.ShowSubcommandHelp(c)
	} else {
		r, err := registry.NewRegistry()
		if err != nil {
			return cli.NewExitError(err.Error(), 1)
		}
		if tag == "" {
			if keep == 0 {
				fmt.Fprintf(c.App.Writer, "You should either specify the tag or how many images you want to keep\n")
				cli.ShowSubcommandHelp(c)
			} else {
				tags, err := r.ListTagsByImage(imgName)

				compareStringNumber := getSortComparisonStrategy(sort)
				Compare(compareStringNumber).Sort(tags)

				if err != nil {
					return cli.NewExitError(err.Error(), 1)
				}
				if len(tags) >= keep {
					for _, tag := range tags[:len(tags)-keep] {
						if dryRun {
							fmt.Printf("%s:%s image would be deleted (Dry Run) ...\n", imgName, tag)
						} else {
							fmt.Printf("%s:%s image will be deleted ...\n", imgName, tag)
							r.DeleteImageByTag(imgName, tag)
						}
					}
				} else {
					fmt.Printf("Only %d images are available\n", len(tags))
				}
			}
		} else {
			err = r.DeleteImageByTag(imgName, tag)
			if err != nil {
				return cli.NewExitError(err.Error(), 1)
			}
		}
	}
	return nil
}

func getSortComparisonStrategy(sort string) func(str1, str2 string) bool {
	var compareStringNumber func(str1, str2 string) bool

	if sort == "default" {
		compareStringNumber = func(str1, str2 string) bool {
			return extractNumberFromString(str1) < extractNumberFromString(str2)
		}
	}

	if sort == "semver" {
		compareStringNumber = func(str1, str2 string) bool {
			if str1 == "latest" {
				return false
			}
			if str2 == "latest" {
				return true
			}
			version1, err1 := semver.Make(str1)
			if err1 != nil {
				fmt.Printf("Error parsing version1: %q\n", err1)
			}
			version2, err2 := semver.Make(str2)
			if err2 != nil {
				fmt.Printf("Error parsing version2: %q\n", err2)
			}
			return version1.LT(version2)
		}
	}

	return compareStringNumber
}
