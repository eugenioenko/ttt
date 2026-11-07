package trash

import "encoding/json"

// Pass the path as data: interpolating it into JavaScript would execute filenames.
// Foundation handles recovery metadata without Finder or Apple Events permission.
const macScript = `function run(argv) {
    ObjC.import('Foundation');
    var error = Ref();
    var trashed = Ref();
    var url = $.NSURL.fileURLWithPath(argv[0]);
    if (!$.NSFileManager.defaultManager.trashItemAtURLResultingItemURLError(url, trashed, error)) {
        throw new Error(ObjC.unwrap(error[0].localizedDescription));
    }
    return JSON.stringify(ObjC.unwrap(trashed[0].path));
}`

func move(path string) error {
	_, err := moveMac(path)
	return err
}

func moveMac(path string) (string, error) {
	output, err := run("/usr/bin/osascript", "-l", "JavaScript", "-e", macScript, path)
	if err != nil {
		return "", err
	}
	var trashed string
	err = json.Unmarshal(output, &trashed)
	return trashed, err
}
