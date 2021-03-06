package pcap

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// FileHeader is the parsed header of a pcap file.
// http://wiki.wireshark.org/Development/LibpcapFileFormat
type FileHeader struct {
	MagicNumber  uint32
	VersionMajor uint16
	VersionMinor uint16
	TimeZone     int32
	SigFigs      uint32
	SnapLen      uint32

	// NOTE: 'Network' property has been changed to `linktype`
	// Please see pcap/pcap.h header file.
	//     Network      uint32
	LinkType uint32
}

// Reader parses pcap files.
type Reader struct {
	buf          io.Reader
	err          error
	fourBytes    []byte
	twoBytes     []byte
	sixteenBytes []byte
	Header       FileHeader
}

// NewReader reads pcap data from an io.Reader.
func NewReader(reader io.Reader) (*Reader, error) {
	r := &Reader{
		buf:          reader,
		fourBytes:    make([]byte, 4),
		twoBytes:     make([]byte, 2),
		sixteenBytes: make([]byte, 16),
	}
	switch magic := r.readUint32(); magic {
	case 0xa1b2c3d4:
		break
	case 0xa1b23c4d:
		break
	default:
		return nil, fmt.Errorf("pcap: bad magic number: %0x", magic)
	}
	r.Header = FileHeader{
		MagicNumber:  0xa1b2c3d4,
		VersionMajor: r.readUint16(),
		VersionMinor: r.readUint16(),
		TimeZone:     r.readInt32(),
		SigFigs:      r.readUint32(),
		SnapLen:      r.readUint32(),
		LinkType:     r.readUint32(),
	}
	return r, nil
}

// Next returns the next packet or nil if no more packets can be read.
func (r *Reader) Next(data []byte) *Packet {
	d := r.sixteenBytes
	r.err = r.read(d)
	if r.err != nil {
		return nil
	}
	timeSec := asUint32(d[0:4])
	timeUsec := asUint32(d[4:8])
	capLen := asUint32(d[8:12])
	origLen := asUint32(d[12:16])

	data = data[:capLen]
	if r.err = r.read(data); r.err != nil {
		return nil
	}
	return &Packet{
		Time:   time.Unix(int64(timeSec), int64(timeUsec)),
		Caplen: capLen,
		Len:    origLen,
		Data:   data,
	}
}

func (r *Reader) read(data []byte) error {
	var err error
	n, err := r.buf.Read(data)
	for err == nil && n != len(data) {
		var chunk int
		chunk, err = r.buf.Read(data[n:])
		n += chunk
	}
	if len(data) == n {
		return nil
	}
	return err
}

func (r *Reader) readUint32() uint32 {
	data := r.fourBytes
	if r.err = r.read(data); r.err != nil {
		return 0
	}
	return asUint32(data)
}

func (r *Reader) readInt32() int32 {
	data := r.fourBytes
	if r.err = r.read(data); r.err != nil {
		return 0
	}
	return int32(asUint32(data))
}

func (r *Reader) readUint16() uint16 {
	data := r.twoBytes
	if r.err = r.read(data); r.err != nil {
		return 0
	}
	return asUint16(data)
}

// Writer writes a pcap file.
type Writer struct {
	writer io.Writer
	buf    []byte
}

// NewWriter creates a Writer that stores output in an io.Writer.
// The FileHeader is written immediately.
func NewWriter(writer io.Writer, header *FileHeader) (*Writer, error) {
	w := &Writer{
		writer: writer,
		buf:    make([]byte, 24),
	}
	binary.LittleEndian.PutUint32(w.buf, header.MagicNumber)
	binary.LittleEndian.PutUint16(w.buf[4:], header.VersionMajor)
	binary.LittleEndian.PutUint16(w.buf[6:], header.VersionMinor)
	binary.LittleEndian.PutUint32(w.buf[8:], uint32(header.TimeZone))
	binary.LittleEndian.PutUint32(w.buf[12:], header.SigFigs)
	binary.LittleEndian.PutUint32(w.buf[16:], header.SnapLen)
	binary.LittleEndian.PutUint32(w.buf[20:], header.LinkType)
	if _, err := writer.Write(w.buf); err != nil {
		return nil, err
	}
	return w, nil
}

// Writer writes a packet to the underlying writer.
func (w *Writer) Write(pkt *Packet) error {
	binary.LittleEndian.PutUint32(w.buf, uint32(pkt.Time.Unix()))
	binary.LittleEndian.PutUint32(w.buf[4:], uint32(pkt.Time.Nanosecond()))
	binary.LittleEndian.PutUint32(w.buf[8:], pkt.Caplen)
	binary.LittleEndian.PutUint32(w.buf[12:], pkt.Len)
	if _, err := w.writer.Write(w.buf[:16]); err != nil {
		return err
	}
	_, err := w.writer.Write(pkt.Data)
	return err
}

func asUint32(data []byte) uint32 {
	return binary.LittleEndian.Uint32(data)
}

func asUint16(data []byte) uint16 {
	return binary.LittleEndian.Uint16(data)
}
