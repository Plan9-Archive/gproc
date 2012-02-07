// 
package filemarshal

import (
	"../typeapply"
	"errors"
	"fmt"
	"os"
	"path"
)

// A File holds on-disk storage.
type File struct {
	CurrentName   string // This is where the Encoder can find the file
	DestName      string // The decoder will place a file in DestDir + DestName (eg. "/tmp/xproc" + "/lib/libc.so.6")
	SymlinkTarget string // this is for symlinks
	file          *os.File
	Uid           int
	Gid           int
	Ftype         int // 0=regular, 1=dir, 2=symlink, >2=something else
	Perm          uint32
}

// NewFile creates a new file referring to f,
// which should be seekable (i.e. not a pipe
// or network connection).
/*
func NewFile(f *os.File) *File {
	fi, _ := os.Stat(f.Name())
	return &File{CurrentName: f.Name(), file: f, Fi: *fi}
}
*/

func (dec decoder) SetDestDir(s string) {
	dec.DestDir = s
}

// File returns the backing file of f.
func (f *File) File() *os.File {
	return f.file
}

// Encoder represents a encoding method.
// Examples include gob.Encoder and json.Encoder.
type Encoder interface {
	// Encode writes an encoded version of x.
	Encode(x interface{}) error
}

// Decoder represents a decoding method.
// Examples include gob.Decoder and json.Decoder.
type Decoder interface {
	Decode(x interface{}) error
}

type encoder struct {
	enc Encoder
}

type decoder struct {
	dec     Decoder
	DestDir string // This is not a great place to put things, but it works for now
}

//  Encode writes a representation of x to the encoder.  We have to be
//  careful about how we do this, because the data type that we decode
//  into does not necessarily match the encoded data type.  Fields may
//  occur in a different order, and some fields may not occur in the
//  final type.  To cope with this, we assume that each Name in an
//  os.File is unique.  We first encode x itself, followed by a list of
//  the file names within it, followed by the data from all those files,
//  in list order, as a sequence of byte slices, terminated with a
//  zero-length slice.
// 
//  When the decoder decodes the value, it can then associate the correct
//  item in the data structure with the correct file stream.
func (enc encoder) Encode(x interface{}) error {
	// TODO some kind of signature so that we can be more
	// robust if we try to Decode a stream that has not
	// been encoded with filemarshal?

	err := enc.enc.Encode(x)
	if err != nil {
		return err
	}
	files := make(map[string]*File)
	var names []string
	typeapply.Do(func(f *File) {
		if f.CurrentName != "" && files[f.CurrentName] == nil {
			names = append(names, f.CurrentName)
			files[f.CurrentName] = f
		}
	},
		x)
	err = enc.enc.Encode(names)
	if err != nil {
		return err
	}
	buf := make([]byte, 8192)
	for _, name := range names {
		off := int64(0)
		f := files[name]
		if f.Ftype != 0 {
			continue
		}
		f.file, err = os.Open(f.CurrentName) // Read from the "CurrentName"
		defer f.file.Close()
		if err != nil {
			return err
		}
		for {
			n, err := f.file.ReadAt(buf, off)
			if n > 0 {
				err = enc.enc.Encode(buf[0:n])
				if err != nil {
					return err
				}
			}
			if err != nil {
				break
			}
			off += int64(n)
		}
		err = enc.enc.Encode([]byte{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (dec decoder) Decode(x interface{}) error {
	err := dec.dec.Decode(x)
	if err != nil {
		return err
	}
	var names []string
	err = dec.dec.Decode(&names)
	if err != nil {
		return err
	}
	files := make(map[string][]*File)
	typeapply.Do(func(f *File) {
		if f != nil && f.CurrentName != "" { // We're only concerned with the destination
			files[f.CurrentName] = append(files[f.CurrentName], f)
		}
	},
		x)

	for _, name := range names {
		if files[name] == nil {
		} else {
			samefiles := files[name]
			f := samefiles[0]

			if f == nil {
				return errors.New("file not found in manifest")
			}
			destname := dec.DestDir + f.DestName

			switch {
			case f.Ftype == 1: // directory
				err = os.MkdirAll(destname, os.FileMode(f.Perm&0777)) //uint32(f.Fi.Mode().Perm())&0777)
				if err != nil {
					err = os.Chown(destname, f.Uid, f.Gid)
				}

			case f.Ftype == 2: //symlink
				dir, _ := path.Split(destname)
				_, err = os.Lstat(dir)
				if err != nil {
					os.MkdirAll(dir, 0777)
					err = nil
				}

				if f.SymlinkTarget[0] == '/' {
					// if the link is absolute we glom on our root prefix
					f.SymlinkTarget = dec.DestDir + f.SymlinkTarget
				}
				err = os.Symlink(f.SymlinkTarget, destname)
				// kinda a weird bug. When not using provate mounts
				// and a symlink already exists it has err ="file exists"
				// but is not == to os.EEXIST.. need to check gocode for actual return
				// its probably not exported either...

			case f.Ftype == 0: // regular file
				dir, _ := path.Split(destname)
				_, err = os.Lstat(dir)
				if err != nil {
					os.MkdirAll(dir, 0777)
					err = nil
				}

				f.file, err = os.OpenFile(destname, os.O_RDWR|os.O_CREATE, 0777)
				//defer f.file.Close()
				if err != nil {
					return err
				}

				//f.Name = f.file.Name()
				err = os.Chown(destname, f.Uid, f.Gid)
				for {
					buf := []byte(nil)
					err = dec.dec.Decode(&buf)
					if err != nil {
						return err
					}
					if len(buf) == 0 {
						break
					}
					_, err = f.file.Write(buf)
					if err != nil {
						return err
					}
				}
				err = f.file.Close()
				if err != nil {
					fmt.Printf("Oh crap error: %s\n", err)
					return err
				}
			}
			for _, g := range samefiles[1:] {
				*g = *f
			}
		}
	}
	return nil
}

type nullWriter struct{}

func (nullWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}
func (nullWriter) Seek(int64, int) (int64, error) {
	return 0, nil
}

// NewEncoder returns a new Encoder that will encode any *File instance
// that it finds within values passed to Encode.  The resulting encoded
// stream must be decoded with a decoder created by NewDecoder.
func NewEncoder(enc Encoder) Encoder {
	return encoder{enc}
}

// NewDecoder returns a new Decoder that can decode an encoding stream
// produced by an encoder created with NewEncoder.  Any File data gets
// written to disk rather than being stored in memory.  When a File is
// decoded, its file name will have changed to the name of a local temporary
// file holding the same data.
// I don't think including the root in the constructor is a great idea,
// but it's the best we've got right now.
func NewDecoder(dec Decoder, root string) Decoder {
	return decoder{dec, root}
}
