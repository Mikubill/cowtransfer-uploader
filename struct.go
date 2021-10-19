package main

type mainConfig struct {
	token      string
	parallel   int
	interval   int
	prefix     string
	debugMode  bool
	singleMode bool
	version    bool
	keepMode   bool
	authCode   string
	blockSize  int
	hashCheck  bool
	passCode   string
	silentMode bool
	validDays  int
}

type uploadResult struct {
	Hash string `json:"hash"`
	Key  string `json:"key"`
}

type initResp struct {
	Token        string
	TransferGUID string
	FileGUID     string
	EncodeID     string
	Exp          int64  `json:"expireAt"`
	ID           string `json:"uploadId"`
}

type upResp struct {
	Etag string `json:"etag"`
	MD5  string `json:"md5"`
}

type prepareSendResp struct {
	UploadToken  string `json:"uptoken"`
	TransferGUID string `json:"transferguid"`
	FileGUID     string `json:"fileguid"`
	UniqueURL    string `json:"uniqueurl"`
	Prefix       string `json:"prefix"`
	QRCode       string `json:"qrcode"`
	Error        bool   `json:"error"`
	ErrorMessage string `json:"error_message"`
}

// type uploadResponse struct {
// 	Ticket string `json:"ctx"`
// 	Hash   int64  `json:"crc32"`
// }

type slek struct {
	ETag string `json:"etag"`
	Part int64  `json:"partNumber"`
}

type clds struct {
	Parts    []slek `json:"parts"`
	FName    string `json:"fname"`
	Mimetype string `json:"mimeType"`
	Metadata map[string]string
	Vars     map[string]string
}

type beforeSendResp struct {
	FileGuid string `json:"fileGuid"`
}

type finishResponse struct {
	TempDownloadCode string `json:"tempDownloadCode"`
	Status           bool   `json:"complete"`
}
