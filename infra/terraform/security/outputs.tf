output "detector_id" {
  description = "The GuardDuty detector, for the console and for exercises."
  value       = aws_guardduty_detector.this.id
}
