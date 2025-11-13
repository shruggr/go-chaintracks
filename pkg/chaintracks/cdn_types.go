package chaintracks

// CDNMetadata represents the JSON metadata file structure
type CDNMetadata struct {
	RootFolder     string          `json:"rootFolder"`
	JSONFilename   string          `json:"jsonFilename"`
	HeadersPerFile int             `json:"headersPerFile"`
	Files          []CDNFileEntry  `json:"files"`
}

// CDNFileEntry represents a single file entry in the metadata
type CDNFileEntry struct {
	Chain         string `json:"chain"`
	Count         int    `json:"count"`
	FileHash      string `json:"fileHash"`
	FileName      string `json:"fileName"`
	FirstHeight   uint32 `json:"firstHeight"`
	LastChainWork string `json:"lastChainWork"`
	LastHash      string `json:"lastHash"`
	PrevChainWork string `json:"prevChainWork"`
	PrevHash      string `json:"prevHash"`
	SourceURL     string `json:"sourceUrl"`
}
