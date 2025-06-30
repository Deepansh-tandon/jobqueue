package tasks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jung-kurt/gofpdf"
	"jobqueue/internal/models"
)

type ReceiptGenerator struct{}

type ReceiptPayload struct {
	To      string  `json:"to"`
	Item    string  `json:"item"`
	Amount  float64 `json:"amount"`
	IsPaid  bool    `json:"is_paid"`
}

func (g *ReceiptGenerator) Process(ctx context.Context, job models.Job) error {
	var p ReceiptPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		// Non-retryable error, payload is malformed.
		return fmt.Errorf("failed to unmarshal receipt payload: %w", err)
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)

	pdf.Cell(40, 10, "Receipt")
	pdf.Ln(20)
	pdf.SetFont("Arial", "", 12)

	pdf.Cell(40, 10, fmt.Sprintf("To: %s", p.To))
	pdf.Ln(10)
	pdf.Cell(40, 10, fmt.Sprintf("Item: %s", p.Item))
	pdf.Ln(10)
	pdf.Cell(40, 10, fmt.Sprintf("Amount: $%.2f", p.Amount))
	pdf.Ln(10)

	status := "Status: UNPAID"
	if p.IsPaid {
		status = "Status: PAID"
	}
	pdf.Cell(40, 10, status)

	// Save to a file named after the job ID. In a real app, this might be uploaded to S3.
	filePath := fmt.Sprintf("receipt-%s.pdf", job.ID)
	err := pdf.OutputFileAndClose(filePath)
	if err != nil {
		return fmt.Errorf("failed to generate PDF: %w", err)
	}

	fmt.Printf("SUCCESS: Generated %s\n", filePath)
	return nil
} 