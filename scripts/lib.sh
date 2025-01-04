eventloop() {
    local line
    while read line; do
        log "line: $line"
        case $line in
        start*|stop*|tick*|memory*|event*) $line ;;
        *) log "unexpected message: $line" ;;
        esac
    done
    log "done reading"
}

# memory ADDR DATA, eg "memory 0 3"
memory() {
    read _ addr data < <(echo $line)
    eval "mem_$addr=$data"
}

# Will appear in the blink11 log
log() {
    echo $* 1>&2
}

emit() {
    log "emit $*"
    echo $*
}
