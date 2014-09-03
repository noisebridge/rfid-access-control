// Just Pseudo-code. To be implemented in go.
class TerminalStub;
class Handler {
    void Init(TerminalStub *stub) = 0;
    void HandleKeypress(char c) = 0;
    void HandleRFID(const string &) = 0;
    void HandleTick() = 0;
};


class TerminalStub {
public:
    void Run(Handler *handler) {
        handler->Init(this);
        for (;;) {
            event_queue_.clear();
            event_queue_.push_back(Readline(500));
            for (line : event_queue_) {
                if (line[0] == 'I') {
                    handler->HandleRFID(line.substr(1));
                } else if (line[0] = 'K') {
                    handler->HandleKeypress(line[1]);
                } else if (line.empty()) {
                    handler->HandleTick();
                } else {
                    // Unexpected event.
                }
            }
        }
    }
   
    bool WriteLCD(int line, string value) {
        // truncate value.
        fprintf(out_, "M%d%s\r\n", line, value);
        return ReadNonEventLine()[0] == 'M';
    }

    string GetTerminalName() {
        fprintf(out_, "n\r\n");
        return ReadNonEventLine().substr(1);
    }

private:
    // Read line from terminal. Skip comment lines.
    string ReadLine(int timeout_ms) {
        for (;;) {
            const string line = in_.readlineWithTimeout(timeout_ms);
            if (line.empty() || line[0] != '#')   // line.empty() -> timeout
                return line;
        }
    }

    // Read line from terminal that is not an event. Events are queued.
    string ReadNonEventLine() {
        for (;;) {
            const string line = ReadLine(-1);
            if (line[0] == 'I' || line[0] = 'K')
                event_queue_.push_back(line);  // keep for later.
            else
                return line;
        }
    }

    list<string> event_queue_;
};
