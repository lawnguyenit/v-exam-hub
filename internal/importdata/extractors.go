package importdata

import (
	"bytes"
	"strings"
)

// contentExtractor is the first seam for format-specific adapters.
// Each extractor only turns an upload into raw text + assets; parsing and
// validation stay in the common pipeline.
type contentExtractor func(data []byte) (string, ExtractInfo, []ExtractedAsset)

func contentExtractors() map[string]contentExtractor {
	return map[string]contentExtractor{
		"txt":  extractTextFile,
		"csv":  extractCSVFile,
		"xml":  extractMoodleXMLFile,
		"docx": extractDocxFile,
		"doc":  extractLegacyDocFile,
		"pdf":  extractPDFFile,
	}
}

func extractTextFile(data []byte) (string, ExtractInfo, []ExtractedAsset) {
	text, extract := normalizeExtractedText("txt", cleanText(data), ExtractInfo{Status: "text_extracted"})
	return text, extract, nil
}

func extractCSVFile(data []byte) (string, ExtractInfo, []ExtractedAsset) {
	text, extract, err := extractCSVQuestions(data)
	if err != nil {
		return "", ExtractInfo{Status: "failed", Warning: "CSV chưa tách được nội dung: " + err.Error()}, nil
	}
	return text, extract, nil
}

func extractMoodleXMLFile(data []byte) (string, ExtractInfo, []ExtractedAsset) {
	text, extract, err := extractMoodleXML(data)
	if err != nil {
		return "", ExtractInfo{Status: "failed", Warning: "XML chưa tách được nội dung: " + err.Error()}, nil
	}
	return text, extract, nil
}

func extractDocxFile(data []byte) (string, ExtractInfo, []ExtractedAsset) {
	text, assets, images, err := extractDocxPackage(data)
	if err != nil {
		return "", ExtractInfo{Status: "failed", Warning: "DOCX chưa tách được nội dung: " + err.Error()}, nil
	}
	text, extract := normalizeExtractedText("docx", text, ExtractInfo{Status: "text_extracted", ImageCount: images})
	return text, extract, assets
}

func extractLegacyDocFile(data []byte) (string, ExtractInfo, []ExtractedAsset) {
	images := inspectEmbeddedImages(data)
	if bytes.HasPrefix(data, oleDocumentHeader) {
		return extractLegacyDoc(data, images)
	}
	return "", ExtractInfo{Status: "unsupported", ImageCount: images, Warning: "File .doc này không đúng header OLE cũ nên server chưa có bộ đọc phù hợp."}, nil
}

func extractPDFFile(data []byte) (string, ExtractInfo, []ExtractedAsset) {
	text, err := extractPDF(data)
	pages, images, fonts := inspectPDF(data)
	if err == nil && strings.TrimSpace(text) != "" {
		text, extract := normalizeExtractedText("pdf", text, ExtractInfo{Status: "text_extracted", PageEstimate: pages, ImageCount: images})
		return text, extract, nil
	}
	if images > 0 && fonts == 0 {
		return "", ExtractInfo{
			Status:       "needs_ocr",
			NeedsOCR:     true,
			ImageCount:   images,
			PageEstimate: pages,
			Warning:      "PDF này giống dạng scan ảnh nên cần OCR trước khi parser local chạy được.",
		}, nil
	}
	return "", ExtractInfo{Status: "failed", PageEstimate: pages, Warning: "PDF chưa tách được text. Cần thêm OCR hoặc bộ extract PDF mạnh hơn."}, nil
}
