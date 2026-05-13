# AWS Fixture Row Notes

The new-service CSV fixtures in this directory are minimal, real-row-derived AWS Bulk Pricing offer rows. They preserve the columns Terracost filters on: `Product Family`, `serviceCode`, `Location`, `Group`, `Group Description`, `usageType`, `operation`, `Unit`, `PricePerUnit`, `StartingRange`, and `EndingRange`.

Representative rows were sampled from AWS public offer CSVs under `https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/<OfferCode>/current/us-east-1/index.csv` in May 2026, then reduced to the dimensions covered by the estimators.

Several fixtures include `DECOY...` SKUs before the intended SKU. These rows share a broad service/group shape but differ by `usageType`, `operation`, or `Unit`; e2e estimation must keep selecting the intended SKU when those decoys are present.
