package spooler

import (
	"github.com/streamsets/sdc2go/api"
	"github.com/streamsets/sdc2go/container/common"
	"github.com/streamsets/sdc2go/container/execution/runner"
	"github.com/streamsets/sdc2go/stages/stagelibrary"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func createStageContext(
	dirPath string,
	processSubDirectories bool,
	filePattern string,
	useLastModified bool,
	initialFileToProcess string,
	pollingTimeoutSeconds int64,
) api.StageContext {
	stageConfig := common.StageConfiguration{}
	stageConfig.Library = LIBRARY
	stageConfig.StageName = STAGE_NAME
	stageConfig.Configuration = make([]common.Config, 6)

	stageConfig.Configuration[0] = common.Config{
		Name:  SPOOL_DIR_PATH,
		Value: dirPath,
	}

	stageConfig.Configuration[1] = common.Config{
		Name:  PROCESS_SUB_DIRECTORIES,
		Value: processSubDirectories,
	}

	stageConfig.Configuration[2] = common.Config{
		Name:  FILE_PATTERN,
		Value: filePattern,
	}

	readOrder := LEXICOGRAPHICAL

	if useLastModified {
		readOrder = LAST_MODIFIED
	}

	stageConfig.Configuration[3] = common.Config{
		Name:  USE_LAST_MODIFIED,
		Value: readOrder,
	}

	stageConfig.Configuration[4] = common.Config{
		Name:  INITIAL_FILE_TO_PROCESS,
		Value: initialFileToProcess,
	}

	stageConfig.Configuration[5] = common.Config{
		Name:  POLLING_TIMEOUT_SECONDS,
		Value: float64(pollingTimeoutSeconds),
	}

	return &common.StageContextImpl{
		StageConfig: stageConfig,
		Parameters:  nil,
	}
}

func createTestDirectory(t *testing.T) string {
	testDir, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatalf("Error happened when creating test Directory, Reason : %s", err.Error())
	}
	t.Logf("Created Test Directory : '%s'", testDir)
	return testDir
}

func deleteTestDirectory(t *testing.T, testDir string) {
	t.Logf("Deleting Test Directory : '%s'", testDir)
	err := os.RemoveAll(testDir)
	if err != nil {
		t.Fatalf(
			"Error happened when deleting test Directory '%s', Reason: %s",
			testDir, err.Error())
	}
}

func createFileAndWriteContents(t *testing.T, filePath string, data string) {
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Error Creating file '%s'", filePath)
	}
	t.Logf("Successfully created File : %s", filePath)
	defer f.Sync()
	defer f.Close()
	_, err = f.WriteString(data)
	if err != nil {
		t.Fatalf("Error Writing to file '%s'", filePath)
	}
}

func createSpoolerAndRun(
	t *testing.T,
	stageContext api.StageContext,
	lastSourceOffset string,
	batchSize int,
) (string, []api.Record) {
	stageInstance, err := stagelibrary.CreateStageInstance(LIBRARY, STAGE_NAME)
	if err != nil {
		t.Fatal(err)
	}
	err = stageInstance.Init(stageContext)
	if err != nil {
		t.Fatal(err)
	}

	batchMaker := runner.NewBatchMakerImpl(runner.StagePipe{})

	offset, err := stageInstance.(api.Origin).Produce(lastSourceOffset, batchSize, batchMaker)
	if err != nil {
		t.Fatal("Err :", err)
	}

	stageInstance.Destroy()

	return offset, batchMaker.GetStageOutput()
}

func checkRecord(
	t *testing.T,
	record api.Record,
	value interface{},
	headersToCheck map[string]string,
) {
	isError := false
	expectedValue := value.(string)

	actualValue := record.GetValue().(string)
	actualHeaders := record.GetHeader().GetAttributes()

	if actualValue != expectedValue {
		isError = true
		t.Errorf(
			"Record value does not match, Expected : '%s', Actual : '%s'",
			expectedValue,
			actualValue,
		)
	}
	for headerName, expectedHeaderVal := range headersToCheck {
		actualHeaderVal := actualHeaders[headerName]
		if actualHeaderVal != expectedHeaderVal {
			isError = true
			t.Errorf(
				"Record Header '%s' does not match, Expected : '%s', Actual : '%s'",
				headerName,
				expectedHeaderVal,
				actualHeaderVal,
			)

		}
	}

	if isError {
		t.Fatalf(
			"Error happened when asserting record values/headers :'%s'",
			record.GetHeader().GetSourceId(),
		)
	}
}

func TestUseLastModified(t *testing.T) {
	testDir := createTestDirectory(t)

	defer deleteTestDirectory(t, testDir)

	//Create a.txt,c.txt,b.txt with different mod times
	createFileAndWriteContents(t, filepath.Join(testDir, "a.txt"), "123\n456")
	createFileAndWriteContents(t, filepath.Join(testDir, "c.txt"), "111112113\n114115116\n117118119")
	createFileAndWriteContents(t, filepath.Join(testDir, "b.txt"), "111213\n141516")

	currentTime := time.Now()

	os.Chtimes(
		filepath.Join(testDir, "a.txt"),
		currentTime, time.Unix(0, currentTime.UnixNano()-(3*time.Second).Nanoseconds()))
	os.Chtimes(
		filepath.Join(testDir, "c.txt"),
		currentTime, time.Unix(0, currentTime.UnixNano()-(2*time.Second).Nanoseconds()))
	os.Chtimes(
		filepath.Join(testDir, "b.txt"),
		currentTime, time.Unix(0, currentTime.UnixNano()-(time.Second).Nanoseconds()))

	stageContext := createStageContext(testDir, false, "*", true, "", 1)

	offset, records := createSpoolerAndRun(t, stageContext, "", 3)

	if len(records) != 2 {
		t.Fatalf("Wrong number of records, Actual : %d, Expected : %d ", len(records), 2)
	}

	expectedHeaders := map[string]string{
		FILE:      filepath.Join(testDir, "a.txt"),
		FILE_NAME: "a.txt",
		OFFSET:    "0",
	}

	checkRecord(t, records[0], "123", expectedHeaders)

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "a.txt"),
		FILE_NAME: "a.txt",
		OFFSET:    "4",
	}

	checkRecord(t, records[1], "456", expectedHeaders)

	offset, records = createSpoolerAndRun(t, stageContext, offset, 2)

	if len(records) != 2 {
		t.Fatalf("Wrong number of records, Actual : %d, Expected : %d ", len(records), 2)
	}

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "c.txt"),
		FILE_NAME: "c.txt",
		OFFSET:    "0",
	}

	checkRecord(t, records[0], "111112113", expectedHeaders)

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "c.txt"),
		FILE_NAME: "c.txt",
		OFFSET:    "10",
	}

	checkRecord(t, records[1], "114115116", expectedHeaders)

	offset, records = createSpoolerAndRun(t, stageContext, offset, 2)

	if len(records) != 1 {
		t.Fatalf("Wrong number of records, Actual : %d, Expected : %d ", len(records), 1)
	}

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "c.txt"),
		FILE_NAME: "c.txt",
		OFFSET:    "20",
	}

	checkRecord(t, records[0], "117118119", expectedHeaders)

	offset, records = createSpoolerAndRun(t, stageContext, offset, 2)

	if len(records) != 2 {
		t.Fatalf("Wrong number of records, Actual : %d, Expected : %d ", len(records), 2)
	}

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "b.txt"),
		FILE_NAME: "b.txt",
		OFFSET:    "0",
	}

	checkRecord(t, records[0], "111213", expectedHeaders)

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "b.txt"),
		FILE_NAME: "b.txt",
		OFFSET:    "7",
	}

	checkRecord(t, records[1], "141516", expectedHeaders)
}

func TestLexicographical(t *testing.T) {

	testDir := createTestDirectory(t)

	defer deleteTestDirectory(t, testDir)

	//Create a.txt,c.txt,b.txt with different mod times
	createFileAndWriteContents(t, filepath.Join(testDir, "a.txt"), "123\n456")
	createFileAndWriteContents(t, filepath.Join(testDir, "b.txt"), "111213\n141516")
	createFileAndWriteContents(t, filepath.Join(testDir, "c.txt"), "111112113\n114115116\n117118119")

	currentTime := time.Now()

	os.Chtimes(
		filepath.Join(testDir, "a.txt"),
		currentTime, time.Unix(0, currentTime.UnixNano()-(3*time.Second).Nanoseconds()))
	os.Chtimes(
		filepath.Join(testDir, "b.txt"),
		currentTime, time.Unix(0, currentTime.UnixNano()-(2*time.Second).Nanoseconds()))
	os.Chtimes(
		filepath.Join(testDir, "c.txt"),
		currentTime, time.Unix(0, currentTime.UnixNano()-(time.Second).Nanoseconds()))

	stageContext := createStageContext(testDir, false, "*", false, "", 1)

	offset, records := createSpoolerAndRun(t, stageContext, "", 3)

	if len(records) != 2 {
		t.Fatalf("Wrong number of records, Actual : %d, Expected : %d ", len(records), 2)
	}

	expectedHeaders := map[string]string{
		FILE:      filepath.Join(testDir, "a.txt"),
		FILE_NAME: "a.txt",
		OFFSET:    "0",
	}

	checkRecord(t, records[0], "123", expectedHeaders)

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "a.txt"),
		FILE_NAME: "a.txt",
		OFFSET:    "4",
	}

	checkRecord(t, records[1], "456", expectedHeaders)

	offset, records = createSpoolerAndRun(t, stageContext, offset, 2)

	if len(records) != 2 {
		t.Fatalf("Wrong number of records, Actual : %d, Expected : %d ", len(records), 2)
	}

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "b.txt"),
		FILE_NAME: "b.txt",
		OFFSET:    "0",
	}

	checkRecord(t, records[0], "111213", expectedHeaders)

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "b.txt"),
		FILE_NAME: "b.txt",
		OFFSET:    "7",
	}

	checkRecord(t, records[1], "141516", expectedHeaders)

	offset, records = createSpoolerAndRun(t, stageContext, offset, 2)

	if len(records) != 2 {
		t.Fatalf("Wrong number of records, Actual : %d, Expected : %d ", len(records), 2)
	}

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "c.txt"),
		FILE_NAME: "c.txt",
		OFFSET:    "0",
	}

	checkRecord(t, records[0], "111112113", expectedHeaders)

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "c.txt"),
		FILE_NAME: "c.txt",
		OFFSET:    "10",
	}

	checkRecord(t, records[1], "114115116", expectedHeaders)

	offset, records = createSpoolerAndRun(t, stageContext, offset, 2)

	if len(records) != 1 {
		t.Fatalf("Wrong number of records, Actual : %d, Expected : %d ", len(records), 1)
	}

	expectedHeaders = map[string]string{
		FILE:      filepath.Join(testDir, "c.txt"),
		FILE_NAME: "c.txt",
		OFFSET:    "20",
	}

	checkRecord(t, records[0], "117118119", expectedHeaders)
}

func TestSubDirectories(t *testing.T) {
	testDir := createTestDirectory(t)
	defer deleteTestDirectory(t, testDir)

	all_letters := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	pathsToCreate := []string{
		"a/b",
		"b/c/d",
		"e/f/g/h",
		"i/j",
		"k/l/m/n",
		"o/p/q/r/s",
		"u",
		"v/w",
		"x/y/z",
	}

	createdFiles := []string{}

	currentTime := time.Now()

	for _, pathToCreate := range pathsToCreate {
		pathToCreate = filepath.Join(testDir, pathToCreate)
		err := os.MkdirAll(pathToCreate, 0777)
		if err != nil {
			t.Fatalf("Error when creating folder: '%s'", pathToCreate)
		}
		fileToCreate := filepath.Join(
			pathToCreate,
			string(all_letters[rand.Intn(len(all_letters)-1)]))
		createFileAndWriteContents(t, fileToCreate, "sample text")
		os.Chtimes(
			fileToCreate, currentTime,
			time.Unix(0, currentTime.UnixNano()+
				(int64(len(createdFiles))*time.Second.Nanoseconds())))
		createdFiles = append(createdFiles, fileToCreate)
	}

	stageContext := createStageContext(testDir, true, "*", true, "", 1)

	var offset string = ""
	var records []api.Record

	for _, fileToCreate := range createdFiles {
		offset, records = createSpoolerAndRun(t, stageContext, offset, 10)

		if len(records) != 1 {
			t.Fatalf(
				"Wrong number of records, Actual : %d, Expected : %d ",
				len(records),
				1,
			)
		}

		expectedHeaders := map[string]string{
			FILE:      fileToCreate,
			FILE_NAME: filepath.Base(fileToCreate),
			OFFSET:    "0",
		}

		checkRecord(t, records[0], "sample text", expectedHeaders)
	}
}
