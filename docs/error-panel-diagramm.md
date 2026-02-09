```mermaid
sequenceDiagram
    actor User
    Turbine ->> HDX Plugin: Produce Error
    HDX Plugin ->> Variables: Set error variable
    Variables ->> Business Text: "Push" errors
    Infinity -->> Business Text: Error templates
    opt Select error
        User ->> Business Text: Select error
        Business Text ->> Variables: Set selected error
        Variables ->> Business Text: "Push" selected error change
    end
    
```
