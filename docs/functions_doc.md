# Functions Documentation

## System Functions

| Function Name | Inputs | Outputs | Description |
| :--- | :--- | :--- | :--- |
| `ExtractTopology` | `*git.Repository` | `[]CommitNode, error` | Traverses active commits mapping them into a strictly chronological visual layout. |
| `Encrypt` | `[]byte, []byte` | `string, error` | Secures raw credentials symmetric to AES-256-GCM. |
| `DecorateCanvas` | `ViewportState` | `WebGL Nodes` | Applies glassmorphism thresholds over the PixiJS layout resolving off-screen interactions. |
