package resources

import (
	"archive/zip"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"errors"
	"io"
)

// zipExeReader attemps to open an executable binary file as a zip file.
// A binary containing a zip file (that isn't a self-extracting binary)
// should contain the file in one of its segments, or appended to the
// end of the file.
func zipExeReader(rda io.ReaderAt, size int64) (*zip.Reader, error) {
	handlers := []func(io.ReaderAt, int64) (*zip.Reader, error){
		zipExeReaderMacho,
		zipExeReaderElf,
		zipExeReaderPe,
	}

	for _, handler := range handlers {
		zfile, err := handler(rda, size)
		if err == nil {
			return zfile, nil
		}
	}
	return nil, errors.New("Couldn't Open As Executable")
}

// zipExeReaderMacho treats the file as a Mach-O binary
// (Mac OS X / Darwin executable) and attempts to find a zip archive.
func zipExeReaderMacho(rda io.ReaderAt, size int64) (*zip.Reader, error) {
	file, err := macho.NewFile(rda)
	if err != nil {
		return nil, err
	}

	var max int64
	for _, load := range file.Loads {
		seg, ok := load.(*macho.Segment)
		if ok {
			// Check if the segment contains a zip file
			if zfile, err := zip.NewReader(seg, int64(seg.Filesz)); err == nil {
				return zfile, nil
			}

			// Otherwise move end of file pointer
			end := int64(seg.Offset + seg.Filesz)
			if end > max {
				max = end
			}
		}
	}

	// No zip file within binary, try appended to end
	section := io.NewSectionReader(rda, max, size-max)
	return zip.NewReader(section, section.Size())
}

// zipExeReaderPe treats the file as a Portable Exectuable binary
// (Windows executable) and attempts to find a zip archive.
func zipExeReaderPe(rda io.ReaderAt, size int64) (*zip.Reader, error) {
	file, err := pe.NewFile(rda)
	if err != nil {
		return nil, err
	}

	var max int64
	for _, sec := range file.Sections {
		// Check if this section has a zip file
		if zfile, err := zip.NewReader(sec, int64(sec.Size)); err == nil {
			return zfile, nil
		}

		// Otherwise move end of file pointer
		end := int64(sec.Offset + sec.Size)
		if end > max {
			max = end
		}
	}

	// No zip file within binary, try appended to end
	section := io.NewSectionReader(rda, max, size-max)
	return zip.NewReader(section, section.Size())
}

// zipExeReaderElf treats the file as a ELF binary
// (linux/BSD/etc... executable) and attempts to find a zip archive.
func zipExeReaderElf(rda io.ReaderAt, size int64) (*zip.Reader, error) {
	file, err := elf.NewFile(rda)
	if err != nil {
		return nil, err
	}

	var max int64
	for _, sect := range file.Sections {
		if sect.Type == elf.SHT_NOBITS {
			continue
		}

		// Check if this section has a zip file
		if sect.ReaderAt != nil {
			if zfile, err := zip.NewReader(sect, int64(sect.Size)); err == nil {
				return zfile, nil
			}
		}

		// Otherwise move end of file pointer
		end := int64(sect.Offset + sect.Size)
		if end > max {
			max = end
		}
	}

	// No zip file within binary, try appended to end
	section := io.NewSectionReader(rda, max, size-max)
	return zip.NewReader(section, section.Size())
}
