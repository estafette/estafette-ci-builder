package builder

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	contracts "github.com/estafette/estafette-ci-contracts"

	"github.com/olekukonko/tablewriter"
)

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func HandleExit(buildLogSteps []*contracts.BuildLogStep) {

	if !contracts.HasSucceededStatus(buildLogSteps) {
		os.Exit(1)
	}

	os.Exit(0)
}

func RenderStats(buildLogSteps []*contracts.BuildLogStep) {

	data := make([][]string, 0)

	dockerPullDurationTotal := 0.0
	dockerRunDurationTotal := 0.0
	dockerImageSizeTotal := int64(0)
	statusTotal := contracts.GetAggregatedStatus(buildLogSteps)

	for _, s := range buildLogSteps {

		// set column values
		stage := s.Step
		image := ""
		imageSize := ""
		imagePullDuration := ""
		stageDuration := fmt.Sprintf("%.0f", s.Duration.Seconds())
		totalDuration := fmt.Sprintf("%.0f", s.Duration.Seconds())
		status := s.Status

		// increment total counters
		dockerRunDurationTotal += s.Duration.Seconds()

		if s.Image != nil {
			// set column values
			image = s.Image.Name
			imageSize = fmt.Sprintf("%v", s.Image.ImageSize/1024/1024)
			imagePullDuration = fmt.Sprintf("%.0f", s.Image.PullDuration.Seconds())
			totalDuration = fmt.Sprintf("%.0f", s.Image.PullDuration.Seconds()+s.Duration.Seconds())

			// increment total counters
			dockerPullDurationTotal += s.Image.PullDuration.Seconds()
			dockerImageSizeTotal += s.Image.ImageSize
		}

		data = append(data, []string{
			stage,
			image,
			imageSize,
			imagePullDuration,
			stageDuration,
			totalDuration,
			string(status),
		})

	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Stage", "Image", "Size (MB)", "Pull (s)", "Run (s)", "Total (s)", "Status"})
	table.SetFooter([]string{"", "Total", fmt.Sprintf("%v", dockerImageSizeTotal/1024/1024), fmt.Sprintf("%.0f", dockerPullDurationTotal), fmt.Sprintf("%.0f", dockerRunDurationTotal), fmt.Sprintf("%.0f", dockerPullDurationTotal+dockerRunDurationTotal), string(statusTotal), ""})
	table.SetBorder(false)
	table.AppendBulk(data)
	table.Render()
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func getContainerImageName(containerImage string) string {

	containerImageName := ""
	if containerImage != "" {
		containerImageArray := strings.Split(containerImage, ":")
		containerImageName = containerImageArray[0]
	}

	return containerImageName
}

func getContainerImageTag(containerImage string) string {

	containerImageTag := ""
	if containerImage != "" {
		containerImageArray := strings.Split(containerImage, ":")
		containerImageTag = "latest"
		if len(containerImageArray) > 1 {
			containerImageTag = containerImageArray[1]
		}
	}

	return containerImageTag
}

var src = rand.NewSource(time.Now().UnixNano()) //nolint:golint,unused

const letterBytes = "abcdefghijklmnopqrstuvwxyz"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

//nolint:golint,unused,deadcode
func generateRandomString(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}
